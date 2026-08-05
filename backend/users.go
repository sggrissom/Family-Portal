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
	vbeam.RegisterProc(app, JoinFamily)
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
	Id       int         `json:"id"`
	Name     string      `json:"name"`
	Email    string      `json:"email"`
	IsAdmin  bool        `json:"isAdmin"`
	FamilyId int         `json:"familyId,omitempty"`
	Families []FamilyRef `json:"families"`
}

// FamilyRef names one family the user belongs to, and what they may do in it.
type FamilyRef struct {
	Id        int         `json:"id"`
	Name      string      `json:"name"`
	Role      AccessLevel `json:"role"`
	IsPrimary bool        `json:"isPrimary"`
}

type FamilyInfoResponse struct {
	Id         int          `json:"id"`
	Name       string       `json:"name"`
	InviteCode string       `json:"inviteCode"`
	Families   []FamilyInfo `json:"families"`
}

// FamilyInfo describes one family the user belongs to, including the code used
// to invite others into it.
type FamilyInfo struct {
	Id         int         `json:"id"`
	Name       string      `json:"name"`
	InviteCode string      `json:"inviteCode"`
	Role       AccessLevel `json:"role"`
	IsPrimary  bool        `json:"isPrimary"`
}

type JoinFamilyRequest struct {
	InviteCode string `json:"inviteCode"`
}

type JoinFamilyResponse struct {
	Success bool         `json:"success"`
	Error   string       `json:"error,omitempty"`
	Auth    AuthResponse `json:"auth,omitempty"`
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
	Id         int       `json:"id"`
	Name       string    `json:"name"`
	InviteCode string    `json:"inviteCode"`
	Creation   time.Time `json:"creation"`
	CreatedBy  int       `json:"createdBy"`
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

// UsersByFamilyIndex: term = family_id, target = user_id
var UsersByFamilyIndex = vbolt.Index(&cfg.Info, "users_by_family", vpack.FInt, vpack.FInt)

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

// GetFamilyUserIds returns all user IDs for a given family. UsersByFamilyIndex
// only tracks each user's primary family, so members who joined the family as a
// secondary one are picked up from FamilyMembership.
func GetFamilyUserIds(tx *vbolt.Tx, familyId int) (userIds []int) {
	if familyId == 0 {
		return
	}
	vbolt.ReadTermTargets(tx, UsersByFamilyIndex, familyId, &userIds, vbolt.Window{})

	seen := make(map[int]bool, len(userIds))
	for _, userId := range userIds {
		seen[userId] = true
	}
	for _, membership := range GetFamilyMemberships(tx, familyId) {
		if membership.UserId != 0 && !seen[membership.UserId] {
			seen[membership.UserId] = true
			userIds = append(userIds, membership.UserId)
		}
	}
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
	// Index user by family
	vbolt.SetTargetSingleTerm(tx, UsersByFamilyIndex, user.Id, user.FamilyId)
	// Record membership alongside the primary family. Nothing reads this yet.
	EnsureMembershipTx(tx, user.Id, user.FamilyId, AccessAdmin)

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

func GetAuthResponseFromUser(tx *vbolt.Tx, user User) AuthResponse {
	resp := AuthResponse{
		Id:       user.Id,
		Name:     user.Name,
		Email:    user.Email,
		IsAdmin:  user.Id == 1, // First user is admin
		FamilyId: user.FamilyId,
		Families: []FamilyRef{},
	}
	if tx == nil {
		return resp
	}
	for _, familyId := range familiesVisibleTo(tx, user) {
		family := GetFamily(tx, familyId)
		if family.Id == 0 {
			continue
		}
		role := AccessAdmin
		if membership, found := FindMembership(tx, user.Id, familyId); found {
			role = membership.Role
		}
		resp.Families = append(resp.Families, FamilyRef{
			Id:        family.Id,
			Name:      family.Name,
			Role:      role,
			IsPrimary: family.Id == user.FamilyId,
		})
	}
	return resp
}

// GetAuthResponseForUser is the form used by the plain HTTP auth handlers,
// which have no transaction of their own to read memberships with.
func GetAuthResponseForUser(user User) (resp AuthResponse) {
	vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
		resp = GetAuthResponseFromUser(tx, user)
	})
	return
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
	// Built before the commit so it sees the membership AddUserTx just wrote.
	auth := GetAuthResponseFromUser(ctx.Tx, user)
	vbolt.TxCommit(ctx.Tx)

	// Return success response
	resp.Success = true
	resp.Auth = auth
	tokenString, tokenErr := generateJwtTokenString(user)
	if tokenErr == nil {
		resp.Token = tokenString
	}
	return
}

func GetAuthContext(ctx *vbeam.Context, req Empty) (resp AuthResponse, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr == nil && user.Id > 0 {
		resp = GetAuthResponseFromUser(ctx.Tx, user)
	}
	return
}

func GetFamilyInfo(ctx *vbeam.Context, req Empty) (resp FamilyInfoResponse, err error) {
	user, err := GetAuthUser(ctx)
	if err != nil {
		return
	}

	familyIds := familiesVisibleTo(ctx.Tx, user)
	if len(familyIds) == 0 {
		err = ErrNoFamily
		return
	}

	resp.Families = []FamilyInfo{}
	for _, familyId := range familyIds {
		family := GetFamily(ctx.Tx, familyId)
		if family.Id == 0 {
			continue
		}
		role := AccessAdmin
		if membership, found := FindMembership(ctx.Tx, user.Id, familyId); found {
			role = membership.Role
		}
		resp.Families = append(resp.Families, FamilyInfo{
			Id:         family.Id,
			Name:       family.Name,
			InviteCode: family.InviteCode,
			Role:       role,
			IsPrimary:  family.Id == user.FamilyId,
		})
	}

	if len(resp.Families) == 0 {
		err = errors.New("Family not found")
		return
	}

	// The top-level fields describe the primary family, which is what callers
	// predating multi-family membership expect to find here.
	primary := resp.Families[0]
	resp.Id = primary.Id
	resp.Name = primary.Name
	resp.InviteCode = primary.InviteCode
	return
}

func JoinFamily(ctx *vbeam.Context, req JoinFamilyRequest) (resp JoinFamilyResponse, err error) {
	// Get authenticated user
	user, err := GetAuthUser(ctx)
	if err != nil {
		resp.Success = false
		resp.Error = "Authentication required"
		return
	}

	// Validate invite code
	if req.InviteCode == "" {
		resp.Success = false
		resp.Error = "Invite code is required"
		return
	}

	// Find family by invite code
	family := GetFamilyByInviteCode(ctx.Tx, req.InviteCode)
	if family.Id == 0 {
		resp.Success = false
		resp.Error = "Invalid invite code"
		return
	}

	// Check if user is already in this family
	if _, alreadyMember := FindMembership(ctx.Tx, user.Id, family.Id); alreadyMember || user.FamilyId == family.Id {
		resp.Success = false
		resp.Error = "You are already a member of this family"
		return
	}

	// Joining adds a family rather than moving between them. The primary
	// family is left alone, so it keeps naming the user's own household and
	// stays the default context for mutations that name no family.
	vbeam.UseWriteTx(ctx)
	EnsureMembershipTx(ctx.Tx, user.Id, family.Id, AccessAdmin)
	if user.FamilyId == 0 {
		user.FamilyId = family.Id
		vbolt.Write(ctx.Tx, UsersBkt, user.Id, &user)
		vbolt.SetTargetSingleTerm(ctx.Tx, UsersByFamilyIndex, user.Id, user.FamilyId)
	}
	// Built before the commit so it sees the membership just written.
	auth := GetAuthResponseFromUser(ctx.Tx, user)
	vbolt.TxCommit(ctx.Tx)

	// Return success response
	resp.Success = true
	resp.Auth = auth
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
