package backend

import (
	"sort"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
)

func RegisterAnalyticsMethods(app *vbeam.Application) {
	vbeam.RegisterProc(app, GetAnalyticsOverview)
	vbeam.RegisterProc(app, GetUserAnalytics)
	vbeam.RegisterProc(app, GetContentAnalytics)
	vbeam.RegisterProc(app, GetSystemAnalytics)
}

type AnalyticsOverviewResponse struct {
	TotalUsers      int                 `json:"totalUsers"`
	TotalFamilies   int                 `json:"totalFamilies"`
	TotalPhotos     int                 `json:"totalPhotos"`
	TotalMilestones int                 `json:"totalMilestones"`
	ActiveUsers7d   int                 `json:"activeUsers7d"`
	ActiveUsers30d  int                 `json:"activeUsers30d"`
	NewUsers7d      int                 `json:"newUsers7d"`
	NewUsers30d     int                 `json:"newUsers30d"`
	RecentActivity  []ActivitySummary   `json:"recentActivity"`
	SystemHealth    SystemHealthSummary `json:"systemHealth"`
}

type ActivitySummary struct {
	Date       string `json:"date"`
	Photos     int    `json:"photos"`
	Milestones int    `json:"milestones"`
	Logins     int    `json:"logins"`
}

type SystemHealthSummary struct {
	PhotosProcessing int `json:"photosProcessing"`
	PhotosFailed     int `json:"photosFailed"`
}

type UserAnalyticsResponse struct {
	RegistrationTrends     []DataPoint         `json:"registrationTrends"`
	LoginActivityTrends    []DataPoint         `json:"loginActivityTrends"`
	FamilySizeDistribution []DistributionPoint `json:"familySizeDistribution"`
	UserEngagement         EngagementMetrics   `json:"userEngagement"`
	TopActiveFamilies      []FamilyActivity    `json:"topActiveFamilies"`
}

type DataPoint struct {
	Date  string `json:"date"`
	Value int    `json:"value"`
}

type DistributionPoint struct {
	Label string `json:"label"`
	Value int    `json:"value"`
}

type EngagementMetrics struct {
	Total         int `json:"total"`
	NeverLoggedIn int `json:"neverLoggedIn"`
	Active7d      int `json:"active7d"`
	Active30d     int `json:"active30d"`
	Dormant90d    int `json:"dormant90d"`
}

type FamilyActivity struct {
	FamilyName      string `json:"familyName"`
	TotalPhotos     int    `json:"totalPhotos"`
	TotalMilestones int    `json:"totalMilestones"`
	LastActive      string `json:"lastActive"`
	Score           int    `json:"score"`
}

type ContentAnalyticsResponse struct {
	PhotoUploadTrends          []DataPoint          `json:"photoUploadTrends"`
	MilestonesByCategory       []DistributionPoint  `json:"milestonesByCategory"`
	ContentPerFamily           []FamilyContentStats `json:"contentPerFamily"`
	PhotoFormats               []DistributionPoint  `json:"photoFormats"`
	AveragePhotosPerPerson     float64              `json:"averagePhotosPerPerson"`
	AverageMilestonesPerPerson float64              `json:"averageMilestonesPerPerson"`
}

type FamilyContentStats struct {
	FamilyName          string  `json:"familyName"`
	Photos              int     `json:"photos"`
	Milestones          int     `json:"milestones"`
	People              int     `json:"people"`
	PhotosPerPerson     float64 `json:"photosPerPerson"`
	MilestonesPerPerson float64 `json:"milestonesPerPerson"`
}

type SystemAnalyticsResponse struct {
	StorageUsage      StorageMetrics     `json:"storageUsage"`
	ProcessingMetrics ProcessingMetrics  `json:"processingMetrics"`
	PhotoFailures     PhotoFailureReport `json:"photoFailures"`
}

type StorageMetrics struct {
	TotalSize       int64       `json:"totalSize"`
	AverageFileSize int64       `json:"averageFileSize"`
	GrowthTrend     []DataPoint `json:"growthTrend"`
}

type ProcessingMetrics struct {
	SuccessRate float64 `json:"successRate"`
	QueueLength int     `json:"queueLength"`
}

type PhotoFailureReport struct {
	Failed         int           `json:"failed"`
	Stuck          int           `json:"stuck"`
	RecentFailures []FailedPhoto `json:"recentFailures"`
}

type FailedPhoto struct {
	Id        int    `json:"id"`
	FilePath  string `json:"filePath"`
	CreatedAt string `json:"createdAt"`
}

func GetAnalyticsOverview(ctx *vbeam.Context, req Empty) (resp AnalyticsOverviewResponse, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}

	now := time.Now()
	weekAgo := now.AddDate(0, 0, -7)
	monthAgo := now.AddDate(0, 0, -30)

	var users []User
	vbolt.IterateAll(ctx.Tx, UsersBkt, func(key int, user User) bool {
		users = append(users, user)
		return true
	})

	var families []Family
	vbolt.IterateAll(ctx.Tx, FamiliesBkt, func(key int, family Family) bool {
		families = append(families, family)
		return true
	})

	var photos []Image
	vbolt.IterateAll(ctx.Tx, ImagesBkt, func(key int, image Image) bool {
		photos = append(photos, image)
		return true
	})

	var milestones []Milestone
	vbolt.IterateAll(ctx.Tx, MilestoneBkt, func(key int, milestone Milestone) bool {
		milestones = append(milestones, milestone)
		return true
	})

	resp.TotalUsers = len(users)
	resp.TotalFamilies = len(families)
	resp.TotalPhotos = len(photos)
	resp.TotalMilestones = len(milestones)

	for _, user := range users {
		if user.LastLogin.After(weekAgo) {
			resp.ActiveUsers7d++
		}
		if user.LastLogin.After(monthAgo) {
			resp.ActiveUsers30d++
		}
		if user.Creation.After(weekAgo) {
			resp.NewUsers7d++
		}
		if user.Creation.After(monthAgo) {
			resp.NewUsers30d++
		}
	}

	activityMap := make(map[string]*ActivitySummary)
	for i := 0; i < 7; i++ {
		date := now.AddDate(0, 0, -i).Format("2006-01-02")
		activityMap[date] = &ActivitySummary{Date: date}
	}

	for _, photo := range photos {
		date := photo.CreatedAt.Format("2006-01-02")
		if activity, exists := activityMap[date]; exists {
			activity.Photos++
		}
	}

	for _, milestone := range milestones {
		date := milestone.CreatedAt.Format("2006-01-02")
		if activity, exists := activityMap[date]; exists {
			activity.Milestones++
		}
	}

	for _, user := range users {
		date := user.LastLogin.Format("2006-01-02")
		if activity, exists := activityMap[date]; exists {
			activity.Logins++
		}
	}

	for i := 6; i >= 0; i-- {
		date := now.AddDate(0, 0, -i).Format("2006-01-02")
		if activity := activityMap[date]; activity != nil {
			resp.RecentActivity = append(resp.RecentActivity, *activity)
		}
	}

	photosProcessing := 0
	photosFailed := 0
	for _, photo := range photos {
		if photo.Status == 1 {
			photosProcessing++
		} else if photo.Status == 2 {
			photosFailed++
		}
	}

	resp.SystemHealth = SystemHealthSummary{
		PhotosProcessing: photosProcessing,
		PhotosFailed:     photosFailed,
	}

	LogInfo(LogCategoryAdmin, "Analytics overview accessed", nil)
	return
}

func GetUserAnalytics(ctx *vbeam.Context, req Empty) (resp UserAnalyticsResponse, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}

	var users []User
	vbolt.IterateAll(ctx.Tx, UsersBkt, func(key int, user User) bool {
		users = append(users, user)
		return true
	})

	var families []Family
	vbolt.IterateAll(ctx.Tx, FamiliesBkt, func(key int, family Family) bool {
		families = append(families, family)
		return true
	})

	now := time.Now()
	registrationMap := make(map[string]int)
	loginMap := make(map[string]int)

	for i := 0; i < 30; i++ {
		date := now.AddDate(0, 0, -i).Format("2006-01-02")
		registrationMap[date] = 0
		loginMap[date] = 0
	}

	for _, user := range users {
		regDate := user.Creation.Format("2006-01-02")
		if _, exists := registrationMap[regDate]; exists {
			registrationMap[regDate]++
		}

		loginDate := user.LastLogin.Format("2006-01-02")
		if _, exists := loginMap[loginDate]; exists {
			loginMap[loginDate]++
		}
	}

	for i := 29; i >= 0; i-- {
		date := now.AddDate(0, 0, -i).Format("2006-01-02")
		resp.RegistrationTrends = append(resp.RegistrationTrends, DataPoint{
			Date:  date,
			Value: registrationMap[date],
		})
		resp.LoginActivityTrends = append(resp.LoginActivityTrends, DataPoint{
			Date:  date,
			Value: loginMap[date],
		})
	}

	familyMembers := make(map[int]map[int]bool)
	countMember := func(familyId int, userId int) {
		if familyId == 0 || userId == 0 {
			return
		}
		if familyMembers[familyId] == nil {
			familyMembers[familyId] = make(map[int]bool)
		}
		familyMembers[familyId][userId] = true
	}
	for _, user := range users {
		countMember(user.FamilyId, user.Id)
	}
	vbolt.IterateAll(ctx.Tx, FamilyMembershipBkt, func(key int, membership FamilyMembership) bool {
		countMember(membership.FamilyId, membership.UserId)
		return true
	})

	familySizes := make(map[int]int)
	for _, family := range families {
		familySizes[len(familyMembers[family.Id])]++
	}

	for size, count := range familySizes {
		if size > 0 {
			resp.FamilySizeDistribution = append(resp.FamilySizeDistribution, DistributionPoint{
				Label: formatFamilySize(size),
				Value: count,
			})
		}
	}

	resp.UserEngagement.Total = len(users)
	for _, user := range users {
		if !user.LastLogin.After(user.Creation.Add(time.Minute)) {
			resp.UserEngagement.NeverLoggedIn++
			continue
		}
		switch {
		case user.LastLogin.After(now.AddDate(0, 0, -7)):
			resp.UserEngagement.Active7d++
			resp.UserEngagement.Active30d++
		case user.LastLogin.After(now.AddDate(0, 0, -30)):
			resp.UserEngagement.Active30d++
		case user.LastLogin.Before(now.AddDate(0, 0, -90)):
			resp.UserEngagement.Dormant90d++
		}
	}

	familyActivityMap := make(map[int]*FamilyActivity)
	for _, family := range families {
		familyActivityMap[family.Id] = &FamilyActivity{
			FamilyName: family.Name,
		}
	}

	vbolt.IterateAll(ctx.Tx, ImagesBkt, func(key int, image Image) bool {
		if activity, exists := familyActivityMap[image.FamilyId]; exists {
			activity.TotalPhotos++
			if image.CreatedAt.After(time.Time{}) {
				if activity.LastActive == "" || image.CreatedAt.Format("2006-01-02") > activity.LastActive {
					activity.LastActive = image.CreatedAt.Format("2006-01-02")
				}
			}
		}
		return true
	})

	vbolt.IterateAll(ctx.Tx, MilestoneBkt, func(key int, milestone Milestone) bool {
		if activity, exists := familyActivityMap[milestone.FamilyId]; exists {
			activity.TotalMilestones++
			if milestone.CreatedAt.After(time.Time{}) {
				if activity.LastActive == "" || milestone.CreatedAt.Format("2006-01-02") > activity.LastActive {
					activity.LastActive = milestone.CreatedAt.Format("2006-01-02")
				}
			}
		}
		return true
	})

	for _, activity := range familyActivityMap {
		if activity.TotalPhotos > 0 || activity.TotalMilestones > 0 {
			activity.Score = activity.TotalPhotos + activity.TotalMilestones*2
			resp.TopActiveFamilies = append(resp.TopActiveFamilies, *activity)
		}
	}

	LogInfo(LogCategoryAdmin, "User analytics accessed", nil)
	return
}

func GetContentAnalytics(ctx *vbeam.Context, req Empty) (resp ContentAnalyticsResponse, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}

	var photos []Image
	vbolt.IterateAll(ctx.Tx, ImagesBkt, func(key int, image Image) bool {
		photos = append(photos, image)
		return true
	})

	var milestones []Milestone
	vbolt.IterateAll(ctx.Tx, MilestoneBkt, func(key int, milestone Milestone) bool {
		milestones = append(milestones, milestone)
		return true
	})

	now := time.Now()
	photoMap := make(map[string]int)
	for i := 0; i < 30; i++ {
		date := now.AddDate(0, 0, -i).Format("2006-01-02")
		photoMap[date] = 0
	}

	for _, photo := range photos {
		date := photo.CreatedAt.Format("2006-01-02")
		if _, exists := photoMap[date]; exists {
			photoMap[date]++
		}
	}

	for i := 29; i >= 0; i-- {
		date := now.AddDate(0, 0, -i).Format("2006-01-02")
		resp.PhotoUploadTrends = append(resp.PhotoUploadTrends, DataPoint{
			Date:  date,
			Value: photoMap[date],
		})
	}

	categoryMap := make(map[string]int)
	for _, milestone := range milestones {
		categoryMap[milestone.Category]++
	}

	for category, count := range categoryMap {
		resp.MilestonesByCategory = append(resp.MilestonesByCategory, DistributionPoint{
			Label: category,
			Value: count,
		})
	}

	formatMap := make(map[string]int)
	for _, photo := range photos {
		formatMap[photo.MimeType]++
	}

	for format, count := range formatMap {
		resp.PhotoFormats = append(resp.PhotoFormats, DistributionPoint{
			Label: format,
			Value: count,
		})
	}

	var families []Family
	vbolt.IterateAll(ctx.Tx, FamiliesBkt, func(key int, family Family) bool {
		families = append(families, family)
		return true
	})

	homePeople := make(map[int]int)
	vbolt.IterateAll(ctx.Tx, PeopleBkt, func(key int, person Person) bool {
		if person.FamilyId == 0 {
			return true
		}
		homePeople[person.FamilyId]++
		return true
	})

	for _, family := range families {
		stats := FamilyContentStats{
			FamilyName: family.Name,
			People:     homePeople[family.Id],
		}

		for _, photo := range photos {
			if photo.FamilyId == family.Id {
				stats.Photos++
			}
		}

		for _, milestone := range milestones {
			if milestone.FamilyId == family.Id {
				stats.Milestones++
			}
		}

		if stats.People > 0 {
			stats.PhotosPerPerson = float64(stats.Photos) / float64(stats.People)
			stats.MilestonesPerPerson = float64(stats.Milestones) / float64(stats.People)
		}

		if stats.Photos > 0 || stats.Milestones > 0 {
			resp.ContentPerFamily = append(resp.ContentPerFamily, stats)
		}
	}

	totalPeople := 0
	for _, count := range homePeople {
		totalPeople += count
	}

	if totalPeople > 0 {
		resp.AveragePhotosPerPerson = float64(len(photos)) / float64(totalPeople)
		resp.AverageMilestonesPerPerson = float64(len(milestones)) / float64(totalPeople)
	}

	LogInfo(LogCategoryAdmin, "Content analytics accessed", nil)
	return
}

func GetSystemAnalytics(ctx *vbeam.Context, req Empty) (resp SystemAnalyticsResponse, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}

	var photos []Image
	vbolt.IterateAll(ctx.Tx, ImagesBkt, func(key int, image Image) bool {
		photos = append(photos, image)
		return true
	})

	var totalSize int64
	for _, photo := range photos {
		totalSize += int64(photo.FileSize)
	}

	averageSize := int64(0)
	if len(photos) > 0 {
		averageSize = totalSize / int64(len(photos))
	}

	growthTrend := calculateStorageGrowthTrend(photos)

	resp.StorageUsage = StorageMetrics{
		TotalSize:       totalSize,
		AverageFileSize: averageSize,
		GrowthTrend:     growthTrend,
	}

	processingCount := 0
	failedCount := 0
	for _, photo := range photos {
		if photo.Status == 1 {
			processingCount++
		} else if photo.Status == 2 {
			failedCount++
		}
	}

	successRate := float64(100)
	if len(photos) > 0 {
		successRate = float64(len(photos)-failedCount) / float64(len(photos)) * 100
	}

	resp.ProcessingMetrics = ProcessingMetrics{
		SuccessRate: successRate,
		QueueLength: processingCount,
	}

	stuckBefore := time.Now().Add(-time.Hour)
	var failures []FailedPhoto
	for _, photo := range photos {
		switch {
		case photo.Status == 2:
			failures = append(failures, FailedPhoto{
				Id:        photo.Id,
				FilePath:  photo.FilePath,
				CreatedAt: photo.CreatedAt.Format("2006-01-02 15:04"),
			})
		case photo.Status == 1 && photo.CreatedAt.Before(stuckBefore):
			resp.PhotoFailures.Stuck++
		}
	}
	sort.Slice(failures, func(i, j int) bool {
		return failures[i].CreatedAt > failures[j].CreatedAt
	})
	if len(failures) > 20 {
		failures = failures[:20]
	}
	resp.PhotoFailures.Failed = failedCount
	resp.PhotoFailures.RecentFailures = failures

	LogInfo(LogCategoryAdmin, "System analytics accessed", nil)
	return
}

func formatFamilySize(size int) string {
	switch size {
	case 0:
		return "0 members"
	case 1:
		return "1 member"
	case 2:
		return "2 members"
	case 3:
		return "3 members"
	case 4:
		return "4 members"
	case 5:
		return "5 members"
	default:
		return "6+ members"
	}
}

func calculateStorageGrowthTrend(photos []Image) []DataPoint {
	now := time.Now()
	dailyStorage := make(map[string]int64)

	for i := 0; i < 30; i++ {
		date := now.AddDate(0, 0, -i).Format("2006-01-02")
		dailyStorage[date] = 0
	}

	for i := 29; i >= 0; i-- {
		date := now.AddDate(0, 0, -i).Format("2006-01-02")
		currentDate := now.AddDate(0, 0, -i)

		var cumulativeSize int64
		for _, photo := range photos {
			if photo.CreatedAt.Before(currentDate) || photo.CreatedAt.Format("2006-01-02") == date {
				cumulativeSize += int64(photo.FileSize)
			}
		}
		dailyStorage[date] = cumulativeSize
	}

	var trend []DataPoint
	for i := 29; i >= 0; i-- {
		date := now.AddDate(0, 0, -i).Format("2006-01-02")
		sizeMB := dailyStorage[date] / (1024 * 1024)
		trend = append(trend, DataPoint{
			Date:  date,
			Value: int(sizeMB),
		})
	}

	return trend
}
