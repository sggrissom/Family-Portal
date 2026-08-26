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

type CreateAccountRequest struct {
	Name                   string `json:"name"`
	Email                  string `json:"email"`
	Password               string `json:"password"`
	ConfirmPassword        string `json:"confirmPassword"`
	FamilyCode             string `json:"familyCode,omitempty"`
	InitialPersonName      string `json:"initialPersonName,omitempty"`
	InitialPersonGender    int    `json:"initialPersonGender,omitempty"`
	InitialPersonBirthdate string `json:"initialPersonBirthdate,omitempty"`
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

var UsersBkt = vbolt.Bucket(&cfg.Info, "users", vpack.FInt, PackUser)
var FamiliesBkt = vbolt.Bucket(&cfg.Info, "families", vpack.FInt, PackFamily)

var PasswdBkt = vbolt.Bucket(&cfg.Info, "passwd", vpack.FInt, vpack.ByteSlice)

var EmailBkt = vbolt.Bucket(&cfg.Info, "email", vpack.StringZ, vpack.Int)

var InviteCodeBkt = vbolt.Bucket(&cfg.Info, "invite_codes", vpack.StringZ, vpack.Int)

var UsersByFamilyIndex = vbolt.Index(&cfg.Info, "users_by_family", vpack.FInt, vpack.FInt)

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

	if req.FamilyCode != "" {
		family := GetFamilyByInviteCode(tx, req.FamilyCode)
		if family.Id != 0 {
			user.FamilyId = family.Id
		}
	}

	if user.FamilyId == 0 {
		family := createFamilyTx(tx, user.Name+"'s Family", user.Id)
		user.FamilyId = family.Id
	}

	vbolt.Write(tx, UsersBkt, user.Id, &user)
	vbolt.Write(tx, PasswdBkt, user.Id, &hash)
	vbolt.Write(tx, EmailBkt, user.Email, &user.Id)
	vbolt.SetTargetSingleTerm(tx, UsersByFamilyIndex, user.Id, user.FamilyId)
	EnsureMembershipTx(tx, user.Id, user.FamilyId, AccessAdmin)

	return user
}

func createFamilyTx(tx *vbolt.Tx, familyName string, createdBy int) Family {
	var family Family
	family.Id = vbolt.NextIntId(tx, FamiliesBkt)
	family.Name = familyName
	family.Creation = time.Now()
	family.CreatedBy = createdBy

	inviteCode := generateUniqueInviteCodeTx(tx)
	family.InviteCode = inviteCode

	vbolt.Write(tx, FamiliesBkt, family.Id, &family)
	vbolt.Write(tx, InviteCodeBkt, inviteCode, &family.Id)

	return family
}

func generateInviteCode() string {
	token, _ := generateToken(4)
	return token[:8]
}

const inviteCodeAttempts = 8

func generateUniqueInviteCodeTx(tx *vbolt.Tx) string {
	var code string
	for range inviteCodeAttempts {
		code = generateInviteCode()
		var existing int
		vbolt.Read(tx, InviteCodeBkt, code, &existing)
		if existing == 0 {
			return code
		}
	}
	LogErrorSimple(LogCategorySystem, "Could not find an unused invite code", map[string]interface{}{
		"attempts": inviteCodeAttempts,
	})
	return code
}

func initialPersonName(req CreateAccountRequest) string {
	if req.InitialPersonName != "" {
		return req.InitialPersonName
	}
	return req.Name
}

func AddInitialPersonForAccountTx(tx *vbolt.Tx, req CreateAccountRequest, familyId int) (Person, error) {
	return AddPersonTx(tx, AddPersonRequest{
		Name:       initialPersonName(req),
		PersonType: int(Parent),
		Gender:     req.InitialPersonGender,
		Birthdate:  req.InitialPersonBirthdate,
	}, familyId)
}

func GetAuthResponseFromUser(tx *vbolt.Tx, user User) AuthResponse {
	resp := AuthResponse{
		Id:       user.Id,
		Name:     user.Name,
		Email:    user.Email,
		IsAdmin:  user.Id == AdminUserId,
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

func GetAuthResponseForUser(user User) (resp AuthResponse) {
	vbolt.WithReadTx(appDb, func(tx *vbolt.Tx) {
		resp = GetAuthResponseFromUser(tx, user)
	})
	return
}

func CreateAccount(ctx *vbeam.Context, req CreateAccountRequest) (resp CreateAccountResponse, err error) {
	if err = validateCreateAccountRequest(req); err != nil {
		resp.Success = false
		resp.Error = err.Error()
		return
	}

	userId := GetUserId(ctx.Tx, req.Email)
	if userId != 0 {
		resp.Success = false
		resp.Error = "Email already registered"
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		resp.Success = false
		resp.Error = "Failed to process password"
		return
	}

	vbeam.UseWriteTx(ctx)
	user := AddUserTx(ctx.Tx, req, hash)
	if req.InitialPersonBirthdate != "" {
		_, personErr := AddInitialPersonForAccountTx(ctx.Tx, req, user.FamilyId)
		if personErr != nil {
			resp.Success = false
			resp.Error = personErr.Error()
			return
		}
	}
	auth := GetAuthResponseFromUser(ctx.Tx, user)
	vbolt.TxCommit(ctx.Tx)

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

	primary := resp.Families[0]
	resp.Id = primary.Id
	resp.Name = primary.Name
	resp.InviteCode = primary.InviteCode
	return
}

func JoinFamily(ctx *vbeam.Context, req JoinFamilyRequest) (resp JoinFamilyResponse, err error) {
	user, err := GetAuthUser(ctx)
	if err != nil {
		resp.Success = false
		resp.Error = "Authentication required"
		return
	}

	if req.InviteCode == "" {
		resp.Success = false
		resp.Error = "Invite code is required"
		return
	}

	family := GetFamilyByInviteCode(ctx.Tx, req.InviteCode)
	if family.Id == 0 {
		resp.Success = false
		resp.Error = "Invalid invite code"
		return
	}

	if _, alreadyMember := FindMembership(ctx.Tx, user.Id, family.Id); alreadyMember || user.FamilyId == family.Id {
		resp.Success = false
		resp.Error = "You are already a member of this family"
		return
	}

	vbeam.UseWriteTx(ctx)
	EnsureMembershipTx(ctx.Tx, user.Id, family.Id, AccessAdmin)
	if user.FamilyId == 0 {
		user.FamilyId = family.Id
		vbolt.Write(ctx.Tx, UsersBkt, user.Id, &user)
		vbolt.SetTargetSingleTerm(ctx.Tx, UsersByFamilyIndex, user.Id, user.FamilyId)
	}
	auth := GetAuthResponseFromUser(ctx.Tx, user)
	vbolt.TxCommit(ctx.Tx)

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
	if req.InitialPersonBirthdate != "" {
		if err := validateAddPersonRequest(AddPersonRequest{
			Name:       initialPersonName(req),
			PersonType: int(Parent),
			Gender:     req.InitialPersonGender,
			Birthdate:  req.InitialPersonBirthdate,
		}); err != nil {
			return err
		}
	}

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
