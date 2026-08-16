package backend

import (
	"errors"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

// Push notifications are the least observable thing the server does: a send is
// queued from chat, handed to APNs on a background goroutine, and either arrives
// on a phone or does not. Nothing about that is visible from the app. The procs
// below expose the three questions worth asking when a notification does not
// show up — is push configured and running, did the device ever register, and
// what did APNs say about the last few sends — plus a way to trigger a send
// without arranging for a second person to post to chat while you are offline.

// PushConfigIssue mirrors ConfigIssue for the wire. ConfigIssue's fields carry
// no JSON tags because it is a startup-log type, so it is restated here rather
// than tagged in place.
type PushConfigIssue struct {
	Setting string `json:"setting"`
	Detail  string `json:"detail"`
}

type GetPushStatusResponse struct {
	Config APNsConfigInfo    `json:"config"`
	Stats  PushWorkerStats   `json:"stats"`
	Issues []PushConfigIssue `json:"issues"`
	// Device counts, so the status card can say whether anything could receive
	// a notification at all.
	TotalDevices    int `json:"totalDevices"`
	ActiveDevices   int `json:"activeDevices"`
	InactiveDevices int `json:"inactiveDevices"`
}

// GetPushStatus reports configuration, worker state, and recent delivery outcomes.
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

// AdminPushDevice is one registered device, with the token masked.
type AdminPushDevice struct {
	Id          int       `json:"id"`
	UserId      int       `json:"userId"`
	UserName    string    `json:"userName"`
	UserEmail   string    `json:"userEmail"`
	TokenHint   string    `json:"tokenHint"`
	Platform    string    `json:"platform"`
	Environment string    `json:"environment"`
	BundleId    string    `json:"bundleId"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
	IsActive    bool      `json:"isActive"`
	// EnvironmentMismatch is true when the device registered against a different
	// APNs environment than the server is now configured for. This is the most
	// common reason a correctly registered device silently receives nothing.
	EnvironmentMismatch bool `json:"environmentMismatch"`
}

type ListPushDevicesResponse struct {
	Devices []AdminPushDevice `json:"devices"`
}

// ListPushDevices returns every registered device across all families.
func ListPushDevices(ctx *vbeam.Context, req Empty) (resp ListPushDevicesResponse, err error) {
	if _, err = requirePushAdmin(ctx); err != nil {
		return
	}

	serverEnv := GetAPNsConfigInfo().Environment

	resp.Devices = []AdminPushDevice{}
	vbolt.IterateAll(ctx.Tx, PushDeviceTokenBkt, func(key int, token PushDeviceToken) bool {
		device := AdminPushDevice{
			Id:          token.Id,
			UserId:      token.UserId,
			TokenHint:   maskPushToken(token.Token),
			Platform:    token.Platform,
			Environment: token.Environment,
			BundleId:    token.BundleId,
			CreatedAt:   token.CreatedAt,
			UpdatedAt:   token.UpdatedAt,
			IsActive:    token.IsActive,
			// Only flag a mismatch when the server has an environment to compare
			// against; an unset one is a configuration problem reported elsewhere.
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
	// UserId is who to notify. Zero means the calling admin, which is the case
	// that matters — you verify against a phone you are holding.
	UserId int `json:"userId"`
	// Message overrides the default body. Optional.
	Message string `json:"message"`
}

type SendTestPushResponse struct {
	Queued bool `json:"queued"`
	// DeviceCount is how many active devices the notification was queued for.
	// Zero with Queued true means the job will find nothing to send to.
	DeviceCount int    `json:"deviceCount"`
	TargetName  string `json:"targetName"`
}

const defaultTestPushMessage = "Test notification from the Family Portal admin panel."

// SendTestPushNotification queues a push to one user's registered devices.
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
		FamilyId:         target.FamilyId,
		SenderId:         admin.Id,
		SenderName:       admin.Name,
		Content:          message,
		RecipientUserIds: []int{target.Id},
		IsTest:           true,
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

// requirePushAdmin resolves the caller and enforces the admin check shared by
// every proc in this file.
func requirePushAdmin(ctx *vbeam.Context) (user User, err error) {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		err = ErrAuthFailure
		return
	}
	if user.Id != 1 {
		err = errors.New("Unauthorized: Admin access required")
		return
	}
	return
}
