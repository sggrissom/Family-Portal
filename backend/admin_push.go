package backend

import (
	"errors"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

type PushConfigIssue struct {
	Setting string `json:"setting"`
	Detail  string `json:"detail"`
}

type GetPushStatusResponse struct {
	Config          APNsConfigInfo    `json:"config"`
	Stats           PushWorkerStats   `json:"stats"`
	Issues          []PushConfigIssue `json:"issues"`
	TotalDevices    int               `json:"totalDevices"`
	ActiveDevices   int               `json:"activeDevices"`
	InactiveDevices int               `json:"inactiveDevices"`
}

func GetPushStatus(ctx *vbeam.Context, req Empty) (resp GetPushStatusResponse, err error) {
	if _, err = requirePushAdmin(ctx); err != nil {
		return
	}

	resp.Config = GetAPNsConfigInfo()
	resp.Stats = GetPushWorkerStats()

	for _, issue := range checkAPNs() {
		resp.Issues = append(resp.Issues, PushConfigIssue{Setting: issue.Setting, Detail: issue.Detail})
	}
	if resp.Issues == nil {
		resp.Issues = []PushConfigIssue{}
	}

	vbolt.IterateAll(ctx.Tx, PushDeviceTokenBkt, func(key int, token PushDeviceToken) bool {
		resp.TotalDevices++
		if token.IsActive {
			resp.ActiveDevices++
		} else {
			resp.InactiveDevices++
		}
		return true
	})

	return
}

type AdminPushDevice struct {
	Id                  int       `json:"id"`
	UserId              int       `json:"userId"`
	UserName            string    `json:"userName"`
	UserEmail           string    `json:"userEmail"`
	TokenHint           string    `json:"tokenHint"`
	Platform            string    `json:"platform"`
	Environment         string    `json:"environment"`
	BundleId            string    `json:"bundleId"`
	CreatedAt           time.Time `json:"createdAt"`
	UpdatedAt           time.Time `json:"updatedAt"`
	IsActive            bool      `json:"isActive"`
	EnvironmentMismatch bool      `json:"environmentMismatch"`
}

type ListPushDevicesResponse struct {
	Devices []AdminPushDevice `json:"devices"`
}

func ListPushDevices(ctx *vbeam.Context, req Empty) (resp ListPushDevicesResponse, err error) {
	if _, err = requirePushAdmin(ctx); err != nil {
		return
	}

	serverEnv := GetAPNsConfigInfo().Environment

	resp.Devices = []AdminPushDevice{}
	vbolt.IterateAll(ctx.Tx, PushDeviceTokenBkt, func(key int, token PushDeviceToken) bool {
		device := AdminPushDevice{
			Id:                  token.Id,
			UserId:              token.UserId,
			TokenHint:           maskPushToken(token.Token),
			Platform:            token.Platform,
			Environment:         token.Environment,
			BundleId:            token.BundleId,
			CreatedAt:           token.CreatedAt,
			UpdatedAt:           token.UpdatedAt,
			IsActive:            token.IsActive,
			EnvironmentMismatch: serverEnv != "" && token.Environment != serverEnv,
		}

		user := GetUser(ctx.Tx, token.UserId)
		if user.Id != 0 {
			device.UserName = user.Name
			device.UserEmail = user.Email
		}

		resp.Devices = append(resp.Devices, device)
		return true
	})

	return
}

type SendTestPushRequest struct {
	UserId  int    `json:"userId"`
	Message string `json:"message"`
}

type SendTestPushResponse struct {
	Queued      bool   `json:"queued"`
	DeviceCount int    `json:"deviceCount"`
	TargetName  string `json:"targetName"`
}

const defaultTestPushMessage = "Test notification from the Family Portal admin panel."

func SendTestPushNotification(ctx *vbeam.Context, req SendTestPushRequest) (resp SendTestPushResponse, err error) {
	admin, err := requirePushAdmin(ctx)
	if err != nil {
		return
	}

	if !IsPushWorkerEnabled() {
		err = errors.New("push worker is not running; check the APNs configuration")
		return
	}

	targetId := req.UserId
	if targetId == 0 {
		targetId = admin.Id
	}

	target := GetUser(ctx.Tx, targetId)
	if target.Id == 0 {
		err = errors.New("user not found")
		return
	}

	devices := GetActiveDeviceTokensForUser(ctx.Tx, target.Id)
	resp.DeviceCount = len(devices)
	resp.TargetName = target.Name

	message := req.Message
	if message == "" {
		message = defaultTestPushMessage
	}

	job := PushNotificationJob{
		Event:            PushEventTest,
		FamilyId:         target.FamilyId,
		SenderId:         admin.Id,
		SenderName:       admin.Name,
		Content:          message,
		RecipientUserIds: []int{target.Id},
	}

	if err = QueuePushNotification(job); err != nil {
		return
	}

	LogInfo(LogCategoryAdmin, "Admin queued test push notification", map[string]interface{}{
		"adminUserId":  admin.Id,
		"targetUserId": target.Id,
		"deviceCount":  resp.DeviceCount,
	})

	resp.Queued = true
	return
}

func requirePushAdmin(ctx *vbeam.Context) (user User, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}
	return
}
