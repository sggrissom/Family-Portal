package backend

import (
	"family/cfg"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"go.hasen.dev/vbeam"
	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

func TestRequireAdminAccess(t *testing.T) {
	testDBPath := "test_admin_access.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	appDb = db

	var adminUser, regularUser User

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		adminReq := CreateAccountRequest{
			Name:            "Admin User",
			Email:           "admin@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(adminReq.Password), bcrypt.DefaultCost)
		adminUser = AddUserTx(tx, adminReq, hash)
		adminUser.Id = 1
		vbolt.Write(tx, UsersBkt, 1, &adminUser)

		regularReq := CreateAccountRequest{
			Name:            "Regular User",
			Email:           "regular@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}
		hash2, _ := bcrypt.GenerateFromPassword([]byte(regularReq.Password), bcrypt.DefaultCost)
		regularUser = AddUserTx(tx, regularReq, hash2)

		vbolt.TxCommit(tx)
	})

	t.Run("Admin user access granted", func(t *testing.T) {
		ctx := &vbeam.Context{}
		vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
			ctx.Tx = tx
			adminToken, _ := generateAuthJwt(adminUser, httptest.NewRecorder())
			ctx.Token = adminToken

			err := requireAdminAccess(ctx)
			if err != nil {
				t.Errorf("Expected no error for admin user, got %v", err)
			}
		})
	})

	t.Run("Regular user access denied", func(t *testing.T) {
		ctx := &vbeam.Context{}
		vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
			ctx.Tx = tx
			regularToken, _ := generateAuthJwt(regularUser, httptest.NewRecorder())
			ctx.Token = regularToken

			err := requireAdminAccess(ctx)
			if err == nil {
				t.Error("Expected error for regular user")
			}

			expectedError := "Unauthorized: Admin access required"
			if err.Error() != expectedError {
				t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
			}
		})
	})

	t.Run("Unauthenticated user access denied", func(t *testing.T) {
		ctx := &vbeam.Context{}
		vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
			ctx.Tx = tx

			err := requireAdminAccess(ctx)
			if err == nil {
				t.Error("Expected error for unauthenticated user")
			}
		})
	})
}

func TestGetAnalyticsOverview(t *testing.T) {
	testDBPath := "test_analytics_overview.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	appDb = db

	var adminUser User
	now := time.Now()
	weekAgo := now.AddDate(0, 0, -7)
	monthAgo := now.AddDate(0, 0, -30)

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		adminReq := CreateAccountRequest{
			Name:            "Admin User",
			Email:           "admin@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(adminReq.Password), bcrypt.DefaultCost)
		adminUser = AddUserTx(tx, adminReq, hash)
		adminUser.Id = 1
		vbolt.Write(tx, UsersBkt, 1, &adminUser)

		families := []Family{
			{Id: 1, Name: "Family One", Creation: monthAgo.AddDate(0, 0, -10)},
			{Id: 2, Name: "Family Two", Creation: weekAgo.AddDate(0, 0, -1)},
		}
		for _, family := range families {
			vbolt.Write(tx, FamiliesBkt, family.Id, &family)
		}

		testUsers := []User{
			{Id: 2, Email: "active@example.com", FamilyId: 1, Creation: weekAgo.AddDate(0, 0, -1), LastLogin: now.AddDate(0, 0, -1)},
			{Id: 3, Email: "recent@example.com", FamilyId: 1, Creation: weekAgo.AddDate(0, 0, 1), LastLogin: weekAgo.AddDate(0, 0, 1)},
			{Id: 4, Email: "monthly@example.com", FamilyId: 2, Creation: monthAgo.AddDate(0, 0, 1), LastLogin: monthAgo.AddDate(0, 0, 5)},
			{Id: 5, Email: "old@example.com", FamilyId: 2, Creation: monthAgo.AddDate(0, 0, -10), LastLogin: monthAgo.AddDate(0, 0, -5)},
		}
		for _, user := range testUsers {
			if user.Id != 1 {
				vbolt.Write(tx, UsersBkt, user.Id, &user)
			}
		}

		photos := []Image{
			{Id: 1, FamilyId: 1, CreatedAt: now.AddDate(0, 0, -1), Status: 0},
			{Id: 2, FamilyId: 1, CreatedAt: now.AddDate(0, 0, -2), Status: 0},
			{Id: 3, FamilyId: 2, CreatedAt: weekAgo.AddDate(0, 0, -5), Status: 1},
			{Id: 4, FamilyId: 2, CreatedAt: monthAgo.AddDate(0, 0, -10), Status: 2},
		}
		for _, photo := range photos {
			vbolt.Write(tx, ImagesBkt, photo.Id, &photo)
		}

		milestones := []Milestone{
			{Id: 1, PersonId: 1, FamilyId: 1, CreatedAt: now.AddDate(0, 0, -1), Category: "development"},
			{Id: 2, PersonId: 1, FamilyId: 1, CreatedAt: now.AddDate(0, 0, -3), Category: "achievement"},
			{Id: 3, PersonId: 2, FamilyId: 2, CreatedAt: weekAgo.AddDate(0, 0, -2), Category: "first"},
		}
		for _, milestone := range milestones {
			vbolt.Write(tx, MilestoneBkt, milestone.Id, &milestone)
		}

		vbolt.TxCommit(tx)
	})

	t.Run("Analytics overview calculation", func(t *testing.T) {
		ctx := &vbeam.Context{}
		vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
			ctx.Tx = tx
			adminToken, _ := generateAuthJwt(adminUser, httptest.NewRecorder())
			ctx.Token = adminToken

			resp, err := GetAnalyticsOverview(ctx, Empty{})
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}

			if resp.TotalUsers != 5 {
				t.Errorf("Expected 5 total users, got %d", resp.TotalUsers)
			}
			if resp.TotalFamilies != 2 {
				t.Errorf("Expected 2 total families, got %d", resp.TotalFamilies)
			}
			if resp.TotalPhotos != 4 {
				t.Errorf("Expected 4 total photos, got %d", resp.TotalPhotos)
			}
			if resp.TotalMilestones != 3 {
				t.Errorf("Expected 3 total milestones, got %d", resp.TotalMilestones)
			}

			if resp.ActiveUsers7d < 0 || resp.ActiveUsers7d > resp.TotalUsers {
				t.Errorf("Active users 7d should be between 0 and %d, got %d", resp.TotalUsers, resp.ActiveUsers7d)
			}
			if resp.ActiveUsers30d < resp.ActiveUsers7d {
				t.Errorf("Active users 30d (%d) should be >= active users 7d (%d)", resp.ActiveUsers30d, resp.ActiveUsers7d)
			}
			if resp.NewUsers7d < 0 || resp.NewUsers7d > resp.TotalUsers {
				t.Errorf("New users 7d should be between 0 and %d, got %d", resp.TotalUsers, resp.NewUsers7d)
			}
			if resp.NewUsers30d < resp.NewUsers7d {
				t.Errorf("New users 30d (%d) should be >= new users 7d (%d)", resp.NewUsers30d, resp.NewUsers7d)
			}

			if len(resp.RecentActivity) != 7 {
				t.Errorf("Expected 7 days of recent activity, got %d", len(resp.RecentActivity))
			}

			for i := 0; i < len(resp.RecentActivity)-1; i++ {
				current := resp.RecentActivity[i].Date
				next := resp.RecentActivity[i+1].Date
				if current <= next {
					break
				}
			}

			if resp.SystemHealth.PhotosProcessing < 0 {
				t.Error("Photos processing count should not be negative")
			}
			if resp.SystemHealth.PhotosFailed < 0 {
				t.Error("Photos failed count should not be negative")
			}
		})
	})

	t.Run("Non-admin cannot access analytics", func(t *testing.T) {
		regularUser := User{Id: 10, Email: "regular@example.com"}

		ctx := &vbeam.Context{}
		vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
			ctx.Tx = tx
			regularToken, _ := generateAuthJwt(regularUser, httptest.NewRecorder())
			ctx.Token = regularToken

			_, err := GetAnalyticsOverview(ctx, Empty{})
			if err == nil {
				t.Error("Expected error for non-admin user")
			}

			expectedError := "Unauthorized: Admin access required"
			if err.Error() != expectedError {
				t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
			}
		})
	})
}

func TestGetUserAnalytics(t *testing.T) {
	testDBPath := "test_user_analytics.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	appDb = db

	var adminUser User

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		adminReq := CreateAccountRequest{
			Name:            "Admin User",
			Email:           "admin@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(adminReq.Password), bcrypt.DefaultCost)
		adminUser = AddUserTx(tx, adminReq, hash)
		adminUser.Id = 1
		vbolt.Write(tx, UsersBkt, 1, &adminUser)

		families := []Family{
			{Id: 1, Name: "Small Family", Creation: time.Now()},
			{Id: 2, Name: "Large Family", Creation: time.Now()},
		}
		for _, family := range families {
			vbolt.Write(tx, FamiliesBkt, family.Id, &family)
		}

		testUsers := []User{
			{Id: 2, Email: "user1@example.com", FamilyId: 1, Creation: time.Now().AddDate(0, 0, -5)},
			{Id: 3, Email: "user2@example.com", FamilyId: 2, Creation: time.Now().AddDate(0, 0, -10)},
			{Id: 4, Email: "user3@example.com", FamilyId: 2, Creation: time.Now().AddDate(0, 0, -15)},
			{Id: 5, Email: "user4@example.com", FamilyId: 2, Creation: time.Now().AddDate(0, 0, -20)},
		}
		for _, user := range testUsers {
			vbolt.Write(tx, UsersBkt, user.Id, &user)
		}

		vbolt.TxCommit(tx)
	})

	t.Run("User analytics calculation", func(t *testing.T) {
		ctx := &vbeam.Context{}
		vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
			ctx.Tx = tx
			adminToken, _ := generateAuthJwt(adminUser, httptest.NewRecorder())
			ctx.Token = adminToken

			resp, err := GetUserAnalytics(ctx, Empty{})
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}

			if len(resp.RegistrationTrends) == 0 {
				t.Error("Expected registration trends data")
			}

			if len(resp.FamilySizeDistribution) == 0 {
				t.Error("Expected family size distribution data")
			}

			for _, point := range resp.FamilySizeDistribution {
				if point.Label == "" {
					t.Error("Distribution point should have a label")
				}
				if point.Value < 0 {
					t.Error("Distribution point value should not be negative")
				}
			}

			eng := resp.UserEngagement
			counted := eng.NeverLoggedIn + eng.Active30d + eng.Dormant90d
			if counted > eng.Total {
				t.Errorf("Engagement buckets (%d) exceed total accounts (%d)", counted, eng.Total)
			}
			if eng.Active7d > eng.Active30d {
				t.Errorf("Active7d (%d) should not exceed Active30d (%d)", eng.Active7d, eng.Active30d)
			}
		})
	})
}

func TestGetContentAnalytics(t *testing.T) {
	testDBPath := "test_content_analytics.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	appDb = db

	var adminUser User

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		adminReq := CreateAccountRequest{
			Name:            "Admin User",
			Email:           "admin@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(adminReq.Password), bcrypt.DefaultCost)
		adminUser = AddUserTx(tx, adminReq, hash)
		adminUser.Id = 1
		vbolt.Write(tx, UsersBkt, 1, &adminUser)

		families := []Family{
			{Id: 1, Name: "Active Family", Creation: time.Now()},
			{Id: 2, Name: "Quiet Family", Creation: time.Now()},
		}
		for _, family := range families {
			vbolt.Write(tx, FamiliesBkt, family.Id, &family)
		}

		people := []Person{
			{Id: 1, FamilyId: 1, Name: "Child One", Type: 1},
			{Id: 2, FamilyId: 1, Name: "Child Two", Type: 1},
			{Id: 3, FamilyId: 2, Name: "Child Three", Type: 1},
		}
		for _, person := range people {
			vbolt.Write(tx, PeopleBkt, person.Id, &person)
		}

		photos := []Image{
			{Id: 1, FamilyId: 1, MimeType: "image/jpeg", CreatedAt: time.Now().AddDate(0, 0, -1)},
			{Id: 2, FamilyId: 1, MimeType: "image/png", CreatedAt: time.Now().AddDate(0, 0, -2)},
			{Id: 3, FamilyId: 1, MimeType: "image/jpeg", CreatedAt: time.Now().AddDate(0, 0, -3)},
			{Id: 4, FamilyId: 2, MimeType: "image/gif", CreatedAt: time.Now().AddDate(0, 0, -4)},
		}
		for _, photo := range photos {
			vbolt.Write(tx, ImagesBkt, photo.Id, &photo)
		}

		milestones := []Milestone{
			{Id: 1, PersonId: 1, FamilyId: 1, Category: "development", CreatedAt: time.Now().AddDate(0, 0, -1)},
			{Id: 2, PersonId: 1, FamilyId: 1, Category: "achievement", CreatedAt: time.Now().AddDate(0, 0, -2)},
			{Id: 3, PersonId: 2, FamilyId: 1, Category: "development", CreatedAt: time.Now().AddDate(0, 0, -3)},
			{Id: 4, PersonId: 3, FamilyId: 2, Category: "first", CreatedAt: time.Now().AddDate(0, 0, -4)},
		}
		for _, milestone := range milestones {
			vbolt.Write(tx, MilestoneBkt, milestone.Id, &milestone)
		}

		vbolt.TxCommit(tx)
	})

	t.Run("Content analytics calculation", func(t *testing.T) {
		ctx := &vbeam.Context{}
		vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
			ctx.Tx = tx
			adminToken, _ := generateAuthJwt(adminUser, httptest.NewRecorder())
			ctx.Token = adminToken

			resp, err := GetContentAnalytics(ctx, Empty{})
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}

			if len(resp.PhotoUploadTrends) == 0 {
				t.Error("Expected photo upload trends data")
			}

			if len(resp.MilestonesByCategory) == 0 {
				t.Error("Expected milestones by category data")
			}

			categoryFound := make(map[string]bool)
			for _, point := range resp.MilestonesByCategory {
				categoryFound[point.Label] = true
				if point.Value <= 0 {
					t.Errorf("Category %s should have positive value, got %d", point.Label, point.Value)
				}
			}

			if len(categoryFound) == 0 {
				t.Error("Expected at least one milestone category")
			}

			if len(resp.PhotoFormats) == 0 {
				t.Error("Expected photo formats data")
			}

			if resp.AveragePhotosPerChild < 0 {
				t.Error("Average photos per child should not be negative")
			}
			if resp.AverageMilestonesPerChild < 0 {
				t.Error("Average milestones per child should not be negative")
			}

			if len(resp.ContentPerFamily) == 0 {
				t.Error("Expected content per family data")
			}

			for _, familyStats := range resp.ContentPerFamily {
				if familyStats.FamilyName == "" {
					t.Error("Family stats should have a name")
				}
				if familyStats.Children < 0 {
					t.Error("Children count should not be negative")
				}
				if familyStats.Photos < 0 {
					t.Error("Photos count should not be negative")
				}
				if familyStats.Milestones < 0 {
					t.Error("Milestones count should not be negative")
				}
			}
		})
	})
}

func TestGetSystemAnalytics(t *testing.T) {
	testDBPath := "test_system_analytics.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	appDb = db

	var adminUser User

	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		adminReq := CreateAccountRequest{
			Name:            "Admin User",
			Email:           "admin@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(adminReq.Password), bcrypt.DefaultCost)
		adminUser = AddUserTx(tx, adminReq, hash)
		adminUser.Id = 1
		vbolt.Write(tx, UsersBkt, 1, &adminUser)
		vbolt.TxCommit(tx)
	})

	t.Run("System analytics structure", func(t *testing.T) {
		ctx := &vbeam.Context{}
		vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
			ctx.Tx = tx
			adminToken, _ := generateAuthJwt(adminUser, httptest.NewRecorder())
			ctx.Token = adminToken

			resp, err := GetSystemAnalytics(ctx, Empty{})
			if err != nil {
				t.Errorf("Expected no error, got %v", err)
			}

			if resp.StorageUsage.TotalSize < 0 {
				t.Error("Total storage size should not be negative")
			}

			if resp.ProcessingMetrics.SuccessRate < 0 {
				t.Error("Success rate should not be negative")
			}
			if resp.PhotoFailures.Failed < 0 || resp.PhotoFailures.Stuck < 0 {
				t.Error("Photo failure counts should not be negative")
			}
			if len(resp.PhotoFailures.RecentFailures) > resp.PhotoFailures.Failed {
				t.Error("Recent failures should be a subset of failed photos")
			}
		})
	})
}

func TestFormatFamilySize(t *testing.T) {
	testCases := []struct {
		size     int
		expected string
	}{
		{1, "1 member"},
		{2, "2 members"},
		{5, "5 members"},
		{0, "0 members"},
	}

	for _, tc := range testCases {
		result := formatFamilySize(tc.size)
		if result != tc.expected {
			t.Errorf("For size %d, expected '%s', got '%s'", tc.size, tc.expected, result)
		}
	}
}

func TestAnalyticsDataStructures(t *testing.T) {
	t.Run("ActivitySummary", func(t *testing.T) {
		activity := ActivitySummary{
			Date:       "2023-06-15",
			Photos:     10,
			Milestones: 5,
			Logins:     3,
		}

		if activity.Date != "2023-06-15" {
			t.Errorf("Expected date '2023-06-15', got '%s'", activity.Date)
		}
		if activity.Photos != 10 {
			t.Errorf("Expected 10 photos, got %d", activity.Photos)
		}
	})

	t.Run("DataPoint", func(t *testing.T) {
		point := DataPoint{
			Date:  "2023-06-15",
			Value: 42,
		}

		if point.Date != "2023-06-15" {
			t.Errorf("Expected date '2023-06-15', got '%s'", point.Date)
		}
		if point.Value != 42 {
			t.Errorf("Expected value 42, got %d", point.Value)
		}
	})

	t.Run("DistributionPoint", func(t *testing.T) {
		point := DistributionPoint{
			Label: "Small Families",
			Value: 15,
		}

		if point.Label != "Small Families" {
			t.Errorf("Expected label 'Small Families', got '%s'", point.Label)
		}
		if point.Value != 15 {
			t.Errorf("Expected value 15, got %d", point.Value)
		}
	})

	t.Run("EngagementMetrics", func(t *testing.T) {
		eng := EngagementMetrics{Total: 10, NeverLoggedIn: 2, Active7d: 3, Active30d: 5, Dormant90d: 1}

		if eng.Active7d > eng.Active30d {
			t.Error("Active7d should be counted within Active30d")
		}
		if eng.NeverLoggedIn+eng.Active30d+eng.Dormant90d > eng.Total {
			t.Error("Buckets should not exceed the total")
		}
	})
}

func TestUserEngagementBuckets(t *testing.T) {
	testDBPath := "test_user_engagement.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	appDb = db
	now := time.Now()

	var adminUser User
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		req := CreateAccountRequest{
			Name:            "Admin User",
			Email:           "admin@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		adminUser = AddUserTx(tx, req, hash)
		adminUser.Id = 1
		adminUser.Creation = now.AddDate(0, 0, -200)
		adminUser.LastLogin = now
		vbolt.Write(tx, UsersBkt, 1, &adminUser)

		created := now.AddDate(0, 0, -30)
		never := User{Id: 2, Name: "Never", Email: "never@example.com", Creation: created, LastLogin: created}
		vbolt.Write(tx, UsersBkt, never.Id, &never)

		lapsed := User{Id: 3, Name: "Lapsed", Email: "lapsed@example.com",
			Creation: now.AddDate(0, 0, -200), LastLogin: now.AddDate(0, 0, -21)}
		vbolt.Write(tx, UsersBkt, lapsed.Id, &lapsed)

		dormant := User{Id: 4, Name: "Dormant", Email: "dormant@example.com",
			Creation: now.AddDate(0, 0, -400), LastLogin: now.AddDate(0, 0, -180)}
		vbolt.Write(tx, UsersBkt, dormant.Id, &dormant)

		vbolt.TxCommit(tx)
	})

	var resp UserAnalyticsResponse
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		ctx := &vbeam.Context{Tx: tx}
		ctx.Token, _ = generateAuthJwt(adminUser, httptest.NewRecorder())

		var err error
		resp, err = GetUserAnalytics(ctx, Empty{})
		if err != nil {
			t.Fatalf("GetUserAnalytics failed: %v", err)
		}
	})

	eng := resp.UserEngagement
	if eng.Total != 4 {
		t.Errorf("Total = %d, want 4", eng.Total)
	}
	if eng.NeverLoggedIn != 1 {
		t.Errorf("NeverLoggedIn = %d, want 1 (the account whose only login is its signup)", eng.NeverLoggedIn)
	}
	if eng.Active7d != 1 {
		t.Errorf("Active7d = %d, want 1", eng.Active7d)
	}
	if eng.Active30d != 2 {
		t.Errorf("Active30d = %d, want 2 (Active7d is counted within it)", eng.Active30d)
	}
	if eng.Dormant90d != 1 {
		t.Errorf("Dormant90d = %d, want 1", eng.Dormant90d)
	}
}
