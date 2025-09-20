package backend

import (
	"errors"
	"family/cfg"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
	"golang.org/x/crypto/bcrypt"
)

func RegisterUserMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, CreateAccount)
	vbeam.RegisterProc(app, GetAuthContext)
	vbeam.RegisterProc(app, GetFamilyInfo)
}

// Request/Response types
type CreateAccountRequest struct {
	Name            string `json:"name"`
	Email           string `json:"email"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirmPassword"`
	FamilyCode      string `json:"familyCode,omitempty"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type CreateAccountResponse struct {
	Success bool         `json:"success"`
	Error   string       `json:"error,omitempty"`
	Token   string       `json:"token,omitempty"`
	Auth    AuthResponse `json:"auth,omitempty"`
}

type LoginResponse struct {
	Success bool         `json:"success"`
	Error   string       `json:"error,omitempty"`
	Token   string       `json:"token,omitempty"`
	Auth    AuthResponse `json:"auth,omitempty"`
}

type AuthResponse struct {
	Id        int    `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	IsAdmin   bool   `json:"isAdmin"`
	FamilyId  int    `json:"familyId,omitempty"`
}

type FamilyInfoResponse struct {
	Id         int    `json:"id"`
	Name       string `json:"name"`
	InviteCode string `json:"inviteCode"`
}

// Database types
type User struct {
	Id        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Creation  time.Time `json:"creation"`
	LastLogin time.Time `json:"lastLogin"`
	FamilyId  int       `json:"familyId"`
}

type Family struct {
	Id           int       `json:"id"`
	Name         string    `json:"name"`
	InviteCode   string    `json:"inviteCode"`
	Creation     time.Time `json:"creation"`
	CreatedBy    int       `json:"createdBy"`
}

// Packing functions for vbolt serialization
func PackUser(self *User, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.String(&self.Name, buf)
	vpack.String(&self.Email, buf)
	vpack.Time(&self.Creation, buf)
	vpack.Time(&self.LastLogin, buf)
	vpack.Int(&self.FamilyId, buf)
}

func PackFamily(self *Family, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.Int(&self.Id, buf)
	vpack.String(&self.Name, buf)
	vpack.String(&self.InviteCode, buf)
	vpack.Time(&self.Creation, buf)
	vpack.Int(&self.CreatedBy, buf)
}

// Buckets for vbolt database storage
var UsersBkt = vbolt.Bucket(&cfg.Info, "users", vpack.FInt, PackUser)
var FamiliesBkt = vbolt.Bucket(&cfg.Info, "families", vpack.FInt, PackFamily)

// user id => hashed password
var PasswdBkt = vbolt.Bucket(&cfg.Info, "passwd", vpack.FInt, vpack.ByteSlice)

// email => user id
var EmailBkt = vbolt.Bucket(&cfg.Info, "email", vpack.StringZ, vpack.Int)

// invite code => family id
var InviteCodeBkt = vbolt.Bucket(&cfg.Info, "invite_codes", vpack.StringZ, vpack.Int)

// Database helper functions
func GetUserId(tx *vbolt.Tx, email string) (userId int) {
	vbolt.Read(tx, EmailBkt, email, &userId)
	return
}

func GetUser(tx *vbolt.Tx, userId int) (user User) {
	vbolt.Read(tx, UsersBkt, userId, &user)
	return
}

func GetPassHash(tx *vbolt.Tx, userId int) (hash []byte) {
	vbolt.Read(tx, PasswdBkt, userId, &hash)
	return
}

func GetFamily(tx *vbolt.Tx, familyId int) (family Family) {
	vbolt.Read(tx, FamiliesBkt, familyId, &family)
	return
}

func GetFamilyByInviteCode(tx *vbolt.Tx, inviteCode string) (family Family) {
	var familyId int
	vbolt.Read(tx, InviteCodeBkt, inviteCode, &familyId)
	if familyId != 0 {
		family = GetFamily(tx, familyId)
	}
	return
}

func AddUserTx(tx *vbolt.Tx, req CreateAccountRequest, hash []byte) User {
	var user User
	user.Id = vbolt.NextIntId(tx, UsersBkt)
	user.Name = req.Name
	user.Email = req.Email
	user.Creation = time.Now()
	user.LastLogin = time.Now()

	// Handle family assignment
	if req.FamilyCode != "" {
		family := GetFamilyByInviteCode(tx, req.FamilyCode)
		if family.Id != 0 {
			user.FamilyId = family.Id
		}
	}

	// If no family or invalid code, create new family
	if user.FamilyId == 0 {
		family := createFamilyTx(tx, user.Name+"'s Family", user.Id)
		user.FamilyId = family.Id
	}

	// Save user data
	vbolt.Write(tx, UsersBkt, user.Id, &user)
	// Store password hash (can be empty for OAuth users)
	vbolt.Write(tx, PasswdBkt, user.Id, &hash)
	vbolt.Write(tx, EmailBkt, user.Email, &user.Id)

	return user
}

func createFamilyTx(tx *vbolt.Tx, familyName string, createdBy int) Family {
	var family Family
	family.Id = vbolt.NextIntId(tx, FamiliesBkt)
	family.Name = familyName
	family.Creation = time.Now()
	family.CreatedBy = createdBy

	// Generate invite code
	inviteCode := generateInviteCode()
	family.InviteCode = inviteCode

	// Save family data
	vbolt.Write(tx, FamiliesBkt, family.Id, &family)
	vbolt.Write(tx, InviteCodeBkt, inviteCode, &family.Id)

	return family
}

func generateInviteCode() string {
	// Generate a simple 8-character invite code
	token, _ := generateToken(4) // 4 bytes = 8 hex characters
	return token[:8]
}

func GetAuthResponseFromUser(user User) AuthResponse {
	return AuthResponse{
		Id:       user.Id,
		Name:     user.Name,
		Email:    user.Email,
		IsAdmin:  user.Id == 1, // First user is admin
		FamilyId: user.FamilyId,
	}
}

// vbeam procedures
func CreateAccount(ctx *vbeam.Context, req CreateAccountRequest) (resp CreateAccountResponse, err error) {
	// Validate request
	if err = validateCreateAccountRequest(req); err != nil {
		resp.Success = false
		resp.Error = err.Error()
		return
	}

	// Check if email already exists
	userId := GetUserId(ctx.Tx, req.Email)
	if userId != 0 {
		resp.Success = false
		resp.Error = "Email already registered"
		return
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		resp.Success = false
		resp.Error = "Failed to process password"
		return
	}

	// Create user
	vbeam.UseWriteTx(ctx)
	user := AddUserTx(ctx.Tx, req, hash)
	vbolt.TxCommit(ctx.Tx)

	// Return success response
	resp.Success = true
	resp.Auth = GetAuthResponseFromUser(user)
	return
}

func GetAuthContext(ctx *vbeam.Context, req Empty) (resp AuthResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr == nil && user.Id > 0 {
		resp = GetAuthResponseFromUser(user)
	}
	return
}

func GetFamilyInfo(ctx *vbeam.Context, req Empty) (resp FamilyInfoResponse, err error) {
	user, err := GetAuthUser(ctx)
	if err != nil {
		return
	}

	if user.FamilyId == 0 {
		err = errors.New("User is not part of a family")
		return
	}

	family := GetFamily(ctx.Tx, user.FamilyId)
	if family.Id == 0 {
		err = errors.New("Family not found")
		return
	}

	resp = FamilyInfoResponse{
		Id:         family.Id,
		Name:       family.Name,
		InviteCode: family.InviteCode,
	}
	return
}

type Empty struct{}

func validateCreateAccountRequest(req CreateAccountRequest) error {
	if req.Name == "" {
		return errors.New("Name is required")
	}
	if req.Email == "" {
		return errors.New("Email is required")
	}

	// Allow empty passwords for OAuth users
	if req.Password != "" {
		if len(req.Password) < 8 {
			return errors.New("Password must be at least 8 characters")
		}
		if req.Password != req.ConfirmPassword {
			return errors.New("Passwords do not match")
		}
	}
	return nil
}