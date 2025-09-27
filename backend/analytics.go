package backend

import (
	"errors"
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

// Analytics Overview
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

// User Analytics
type UserAnalyticsResponse struct {
	RegistrationTrends    []DataPoint       `json:"registrationTrends"`
	LoginActivityTrends   []DataPoint       `json:"loginActivityTrends"`
	FamilySizeDistribution []DistributionPoint `json:"familySizeDistribution"`
	UserRetention         RetentionMetrics  `json:"userRetention"`
	TopActiveFamilies     []FamilyActivity  `json:"topActiveFamilies"`
}

type DataPoint struct {
	Date  string `json:"date"`
	Value int    `json:"value"`
}

type DistributionPoint struct {
	Label string `json:"label"`
	Value int    `json:"value"`
}

type RetentionMetrics struct {
	Day1    float64 `json:"day1"`
	Day7    float64 `json:"day7"`
	Day30   float64 `json:"day30"`
	Day90   float64 `json:"day90"`
}

type FamilyActivity struct {
	FamilyName  string `json:"familyName"`
	TotalPhotos int    `json:"totalPhotos"`
	TotalMilestones int `json:"totalMilestones"`
	LastActive  string `json:"lastActive"`
	Score       int    `json:"score"`
}

// Content Analytics
type ContentAnalyticsResponse struct {
	PhotoUploadTrends      []DataPoint         `json:"photoUploadTrends"`
	MilestonesByCategory   []DistributionPoint `json:"milestonesByCategory"`
	ContentPerFamily       []FamilyContentStats `json:"contentPerFamily"`
	PhotoFormats           []DistributionPoint `json:"photoFormats"`
	AveragePhotosPerChild  float64             `json:"averagePhotosPerChild"`
	AverageMilestonesPerChild float64         `json:"averageMilestonesPerChild"`
}

type FamilyContentStats struct {
	FamilyName  string `json:"familyName"`
	Photos      int    `json:"photos"`
	Milestones  int    `json:"milestones"`
	Children    int    `json:"children"`
	PhotosPerChild     float64 `json:"photosPerChild"`
	MilestonesPerChild float64 `json:"milestonesPerChild"`
}

// System Analytics
type SystemAnalyticsResponse struct {
	StorageUsage        StorageMetrics     `json:"storageUsage"`
	ProcessingMetrics   ProcessingMetrics  `json:"processingMetrics"`
	ErrorAnalysis       ErrorAnalysis      `json:"errorAnalysis"`
	APIRequestTrends    []DataPoint        `json:"apiRequestTrends"`
}

type StorageMetrics struct {
	TotalSize       int64   `json:"totalSize"`
	AverageFileSize int64   `json:"averageFileSize"`
	GrowthTrend     []DataPoint `json:"growthTrend"`
}

type ProcessingMetrics struct {
	SuccessRate        float64 `json:"successRate"`
	AverageProcessTime float64 `json:"averageProcessTime"`
	QueueLength        int     `json:"queueLength"`
}

type ErrorAnalysis struct {
	TotalErrors      int                 `json:"totalErrors"`
	ErrorsByCategory []DistributionPoint `json:"errorsByCategory"`
	ErrorsByLevel    []DistributionPoint `json:"errorsByLevel"`
	RecentErrors     []string            `json:"recentErrors"`
}

// Helper function to check admin access
func requireAdminAccess(ctx *vbeam.Context) error {
	user, authErr := GetAuthUser(ctx)
	if authErr != nil {
		return ErrAuthFailure
	}
	if user.Id != 1 {
		return errors.New("Unauthorized: Admin access required")
	}
	return nil
}

// Get analytics overview with key metrics
func GetAnalyticsOverview(ctx *vbeam.Context, req Empty) (resp AnalyticsOverviewResponse, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}

	now := time.Now()
	weekAgo := now.AddDate(0, 0, -7)
	monthAgo := now.AddDate(0, 0, -30)

	// Count totals
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

	// Calculate active users
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

	// Recent activity (last 7 days)
	activityMap := make(map[string]*ActivitySummary)
	for i := 0; i < 7; i++ {
		date := now.AddDate(0, 0, -i).Format("2006-01-02")
		activityMap[date] = &ActivitySummary{Date: date}
	}

	// Count photos by day
	for _, photo := range photos {
		date := photo.CreatedAt.Format("2006-01-02")
		if activity, exists := activityMap[date]; exists {
			activity.Photos++
		}
	}

	// Count milestones by day
	for _, milestone := range milestones {
		date := milestone.CreatedAt.Format("2006-01-02")
		if activity, exists := activityMap[date]; exists {
			activity.Milestones++
		}
	}

	// Count logins by day (approximation using LastLogin)
	for _, user := range users {
		date := user.LastLogin.Format("2006-01-02")
		if activity, exists := activityMap[date]; exists {
			activity.Logins++
		}
	}

	// Convert to slice
	for i := 6; i >= 0; i-- {
		date := now.AddDate(0, 0, -i).Format("2006-01-02")
		if activity := activityMap[date]; activity != nil {
			resp.RecentActivity = append(resp.RecentActivity, *activity)
		}
	}

	// System health
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

// Get detailed user analytics
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

	// Registration trends (last 30 days)
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

	// Convert to data points
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

	// Family size distribution
	familySizes := make(map[int]int) // family size -> count
	for _, family := range families {
		size := 0
		for _, user := range users {
			if user.FamilyId == family.Id {
				size++
			}
		}
		familySizes[size]++
	}

	for size, count := range familySizes {
		if size > 0 {
			resp.FamilySizeDistribution = append(resp.FamilySizeDistribution, DistributionPoint{
				Label: formatFamilySize(size),
				Value: count,
			})
		}
	}

	// Calculate retention (simplified)
	totalUsers := len(users)
	if totalUsers > 0 {
		day1Retained := 0
		day7Retained := 0
		day30Retained := 0
		day90Retained := 0

		for _, user := range users {
			daysSinceReg := int(now.Sub(user.Creation).Hours() / 24)
			daysSinceLogin := int(now.Sub(user.LastLogin).Hours() / 24)

			if daysSinceReg >= 1 && daysSinceLogin <= 1 {
				day1Retained++
			}
			if daysSinceReg >= 7 && daysSinceLogin <= 7 {
				day7Retained++
			}
			if daysSinceReg >= 30 && daysSinceLogin <= 30 {
				day30Retained++
			}
			if daysSinceReg >= 90 && daysSinceLogin <= 90 {
				day90Retained++
			}
		}

		resp.UserRetention = RetentionMetrics{
			Day1:  float64(day1Retained) / float64(totalUsers) * 100,
			Day7:  float64(day7Retained) / float64(totalUsers) * 100,
			Day30: float64(day30Retained) / float64(totalUsers) * 100,
			Day90: float64(day90Retained) / float64(totalUsers) * 100,
		}
	}

	// Top active families (by content creation)
	familyActivityMap := make(map[int]*FamilyActivity)
	for _, family := range families {
		familyActivityMap[family.Id] = &FamilyActivity{
			FamilyName: family.Name,
		}
	}

	// Count photos per family
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

	// Count milestones per family
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

	// Calculate activity scores and add to response
	for _, activity := range familyActivityMap {
		if activity.TotalPhotos > 0 || activity.TotalMilestones > 0 {
			activity.Score = activity.TotalPhotos + activity.TotalMilestones*2 // Weight milestones higher
			resp.TopActiveFamilies = append(resp.TopActiveFamilies, *activity)
		}
	}

	LogInfo(LogCategoryAdmin, "User analytics accessed", nil)
	return
}

// Get detailed content analytics
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

	// Photo upload trends (last 30 days)
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

	// Milestones by category
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

	// Photo formats
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

	// Content per family
	var families []Family
	vbolt.IterateAll(ctx.Tx, FamiliesBkt, func(key int, family Family) bool {
		families = append(families, family)
		return true
	})

	for _, family := range families {
		stats := FamilyContentStats{
			FamilyName: family.Name,
		}

		// Count photos and milestones for this family
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

		// Count children in this family
		vbolt.IterateAll(ctx.Tx, PeopleBkt, func(key int, person Person) bool {
			if person.FamilyId == family.Id && person.Type == Child {
				stats.Children++
			}
			return true
		})

		if stats.Children > 0 {
			stats.PhotosPerChild = float64(stats.Photos) / float64(stats.Children)
			stats.MilestonesPerChild = float64(stats.Milestones) / float64(stats.Children)
		}

		if stats.Photos > 0 || stats.Milestones > 0 {
			resp.ContentPerFamily = append(resp.ContentPerFamily, stats)
		}
	}

	// Calculate averages
	totalChildren := 0
	for _, stats := range resp.ContentPerFamily {
		totalChildren += stats.Children
	}

	if totalChildren > 0 {
		resp.AveragePhotosPerChild = float64(len(photos)) / float64(totalChildren)
		resp.AverageMilestonesPerChild = float64(len(milestones)) / float64(totalChildren)
	}

	LogInfo(LogCategoryAdmin, "Content analytics accessed", nil)
	return
}

// Get system analytics
func GetSystemAnalytics(ctx *vbeam.Context, req Empty) (resp SystemAnalyticsResponse, err error) {
	if err = requireAdminAccess(ctx); err != nil {
		return
	}

	var photos []Image
	vbolt.IterateAll(ctx.Tx, ImagesBkt, func(key int, image Image) bool {
		photos = append(photos, image)
		return true
	})

	// Storage metrics
	var totalSize int64
	for _, photo := range photos {
		totalSize += int64(photo.FileSize)
	}

	averageSize := int64(0)
	if len(photos) > 0 {
		averageSize = totalSize / int64(len(photos))
	}

	resp.StorageUsage = StorageMetrics{
		TotalSize:       totalSize,
		AverageFileSize: averageSize,
		GrowthTrend:     []DataPoint{}, // TODO: Implement storage growth tracking
	}

	// Processing metrics
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
		SuccessRate:        successRate,
		AverageProcessTime: 0, // TODO: Track processing times
		QueueLength:        processingCount,
	}

	// Error analysis (simplified - would need log analysis for full implementation)
	resp.ErrorAnalysis = ErrorAnalysis{
		TotalErrors:      failedCount,
		ErrorsByCategory: []DistributionPoint{},
		ErrorsByLevel:    []DistributionPoint{},
		RecentErrors:     []string{},
	}

	LogInfo(LogCategoryAdmin, "System analytics accessed", nil)
	return
}

// Helper functions
func formatFamilySize(size int) string {
	switch size {
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