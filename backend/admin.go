package backend

import (
	"errors"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func RegisterAdminMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, ListAllUsers)
}

type AdminUserInfo struct {
	Id         int       `json:"id"`
	Name       string    `json:"name"`
	Email      string    `json:"email"`
	Creation   time.Time `json:"creation"`
	LastLogin  time.Time `json:"lastLogin"`
	FamilyId   int       `json:"familyId"`
	FamilyName string    `json:"familyName"`
	IsAdmin    bool      `json:"isAdmin"`
}

type ListAllUsersResponse struct {
	Users []AdminUserInfo `json:"users"`
}

// Admin-only procedure to list all registered users
func ListAllUsers(ctx *vbeam.Context, req Empty) (resp ListAllUsersResponse, err error) {
	// Get authenticated user
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}

	// Check if user is admin (ID == 1)
	if user.Id != 1 {
		err = errors.New("Unauthorized: Admin access required")
		return
	}

	// Get all users using IterateAll
	var users []User
	vbolt.IterateAll(ctx.Tx, UsersBkt, func(key int, user User) bool {
		users = append(users, user)
		return true // Continue iteration
	})

	// Convert to AdminUserInfo with family names
	resp.Users = make([]AdminUserInfo, 0, len(users))
	for _, u := range users {
		familyName := ""
		if u.FamilyId != 0 {
			family := GetFamily(ctx.Tx, u.FamilyId)
			familyName = family.Name
		}

		adminUser := AdminUserInfo{
			Id:         u.Id,
			Name:       u.Name,
			Email:      u.Email,
			Creation:   u.Creation,
			LastLogin:  u.LastLogin,
			FamilyId:   u.FamilyId,
			FamilyName: familyName,
			IsAdmin:    u.Id == 1, // Admin check
		}
		resp.Users = append(resp.Users, adminUser)
	}

	return
}
