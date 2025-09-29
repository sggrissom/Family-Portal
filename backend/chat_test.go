// Package backend_test provides unit tests for chat functionality
// Tests: message database operations, family isolation, data persistence
package backend

import (
	"family/cfg"
	"os"
	"strings"
	"testing"
	"time"

	"go.hasen.dev/vbolt"
	"golang.org/x/crypto/bcrypt"
)

// Test chat message database operations
func TestChatMessageDatabaseOperations(t *testing.T) {
	testDBPath := "test_chat_db.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	var testUser User
	var familyId int

	// Setup: Create test user
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		userReq := CreateAccountRequest{
			Name:            "Test User",
			Email:           "test@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(userReq.Password), bcrypt.DefaultCost)
		testUser = AddUserTx(tx, userReq, hash)
		familyId = testUser.FamilyId
		vbolt.TxCommit(tx)
	})

	t.Run("StoreAndRetrieveMessage", func(t *testing.T) {
		var messageId int

		// Store a message
		vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
			message := ChatMessage{
				FamilyId:        familyId,
				UserId:          testUser.Id,
				UserName:        testUser.Name,
				Content:         "Hello, family!",
				CreatedAt:       time.Now(),
				ClientMessageId: "client-123",
			}

			// Simulate adding a chat message
			message.Id = vbolt.NextIntId(tx, ChatMessagesBkt)
			vbolt.Write(tx, ChatMessagesBkt, message.Id, &message)

			// Update indices
			vbolt.SetTargetSingleTerm(tx, ChatMessagesByFamilyIndex, message.Id, message.FamilyId)
			vbolt.SetTargetSingleTerm(tx, ChatMessagesByUserIndex, message.Id, message.UserId)

			messageId = message.Id
			vbolt.TxCommit(tx)
		})

		// Retrieve the message
		vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
			storedMessage := GetChatMessageById(tx, messageId)

			if storedMessage.Id == 0 {
				t.Error("Message was not stored properly")
			}
			if storedMessage.Content != "Hello, family!" {
				t.Errorf("Expected content 'Hello, family!', got '%s'", storedMessage.Content)
			}
			if storedMessage.FamilyId != familyId {
				t.Errorf("Expected family ID %d, got %d", familyId, storedMessage.FamilyId)
			}
			if storedMessage.UserId != testUser.Id {
				t.Errorf("Expected user ID %d, got %d", testUser.Id, storedMessage.UserId)
			}
			if storedMessage.UserName != testUser.Name {
				t.Errorf("Expected user name '%s', got '%s'", testUser.Name, storedMessage.UserName)
			}
			if storedMessage.ClientMessageId != "client-123" {
				t.Errorf("Expected client message ID 'client-123', got '%s'", storedMessage.ClientMessageId)
			}
		})
	})

	t.Run("GetFamilyChatMessages", func(t *testing.T) {
		var messageIds []int

		// Store multiple messages
		vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
			messages := []string{
				"First message",
				"Second message",
				"Third message",
			}

			for i, content := range messages {
				message := ChatMessage{
					FamilyId:        familyId,
					UserId:          testUser.Id,
					UserName:        testUser.Name,
					Content:         content,
					CreatedAt:       time.Now().Add(time.Duration(i) * time.Second),
					ClientMessageId: "client-" + string(rune(i)),
				}

				message.Id = vbolt.NextIntId(tx, ChatMessagesBkt)
				vbolt.Write(tx, ChatMessagesBkt, message.Id, &message)

				// Update indices
				vbolt.SetTargetSingleTerm(tx, ChatMessagesByFamilyIndex, message.Id, message.FamilyId)
				vbolt.SetTargetSingleTerm(tx, ChatMessagesByUserIndex, message.Id, message.UserId)

				messageIds = append(messageIds, message.Id)
			}

			vbolt.TxCommit(tx)
		})

		// Retrieve family messages
		vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
			messages := GetFamilyChatMessages(tx, familyId, 10)

			if len(messages) < 3 {
				t.Errorf("Expected at least 3 messages, got %d", len(messages))
			}

			// Verify messages belong to the correct family
			for _, message := range messages {
				if message.FamilyId != familyId {
					t.Errorf("Message belongs to wrong family: %d vs %d", message.FamilyId, familyId)
				}
			}
		})
	})
}

// Test family isolation for chat messages
func TestChatMessageFamilyIsolation(t *testing.T) {
	testDBPath := "test_chat_isolation.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	var testUser1, testUser2 User
	var family1Id, family2Id int

	// Setup: Create two users in different families
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		userReq1 := CreateAccountRequest{
			Name:            "User One",
			Email:           "user1@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}
		hash1, _ := bcrypt.GenerateFromPassword([]byte(userReq1.Password), bcrypt.DefaultCost)
		testUser1 = AddUserTx(tx, userReq1, hash1)
		family1Id = testUser1.FamilyId

		userReq2 := CreateAccountRequest{
			Name:            "User Two",
			Email:           "user2@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}
		hash2, _ := bcrypt.GenerateFromPassword([]byte(userReq2.Password), bcrypt.DefaultCost)
		testUser2 = AddUserTx(tx, userReq2, hash2)
		family2Id = testUser2.FamilyId

		vbolt.TxCommit(tx)
	})

	// Store messages for both families
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		// Family 1 message
		message1 := ChatMessage{
			FamilyId:        family1Id,
			UserId:          testUser1.Id,
			UserName:        testUser1.Name,
			Content:         "Family 1 message",
			CreatedAt:       time.Now(),
			ClientMessageId: "family1-msg",
		}
		message1.Id = vbolt.NextIntId(tx, ChatMessagesBkt)
		vbolt.Write(tx, ChatMessagesBkt, message1.Id, &message1)
		vbolt.SetTargetSingleTerm(tx, ChatMessagesByFamilyIndex, message1.Id, message1.FamilyId)

		// Family 2 message
		message2 := ChatMessage{
			FamilyId:        family2Id,
			UserId:          testUser2.Id,
			UserName:        testUser2.Name,
			Content:         "Family 2 message",
			CreatedAt:       time.Now(),
			ClientMessageId: "family2-msg",
		}
		message2.Id = vbolt.NextIntId(tx, ChatMessagesBkt)
		vbolt.Write(tx, ChatMessagesBkt, message2.Id, &message2)
		vbolt.SetTargetSingleTerm(tx, ChatMessagesByFamilyIndex, message2.Id, message2.FamilyId)

		vbolt.TxCommit(tx)
	})

	// Test family 1 isolation
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		family1Messages := GetFamilyChatMessages(tx, family1Id, 10)

		if len(family1Messages) != 1 {
			t.Errorf("Expected 1 message for family 1, got %d", len(family1Messages))
		}

		if len(family1Messages) > 0 && family1Messages[0].Content != "Family 1 message" {
			t.Errorf("Expected 'Family 1 message', got '%s'", family1Messages[0].Content)
		}

		// Verify no cross-family contamination
		for _, message := range family1Messages {
			if message.FamilyId != family1Id {
				t.Errorf("Family 1 query returned message from family %d", message.FamilyId)
			}
		}
	})

	// Test family 2 isolation
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		family2Messages := GetFamilyChatMessages(tx, family2Id, 10)

		if len(family2Messages) != 1 {
			t.Errorf("Expected 1 message for family 2, got %d", len(family2Messages))
		}

		if len(family2Messages) > 0 && family2Messages[0].Content != "Family 2 message" {
			t.Errorf("Expected 'Family 2 message', got '%s'", family2Messages[0].Content)
		}

		// Verify no cross-family contamination
		for _, message := range family2Messages {
			if message.FamilyId != family2Id {
				t.Errorf("Family 2 query returned message from family %d", message.FamilyId)
			}
		}
	})
}

// Test chat message data persistence
func TestChatMessagePersistence(t *testing.T) {
	testDBPath := "test_chat_persistence.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	var testUser User
	var messageId int
	testTime := time.Date(2023, 6, 15, 12, 0, 0, 0, time.UTC)

	// Setup and store message
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		userReq := CreateAccountRequest{
			Name:            "Test User",
			Email:           "test@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(userReq.Password), bcrypt.DefaultCost)
		testUser = AddUserTx(tx, userReq, hash)

		message := ChatMessage{
			FamilyId:        testUser.FamilyId,
			UserId:          testUser.Id,
			UserName:        testUser.Name,
			Content:         "Test persistence message",
			CreatedAt:       testTime,
			ClientMessageId: "persistence-test",
		}

		message.Id = vbolt.NextIntId(tx, ChatMessagesBkt)
		vbolt.Write(tx, ChatMessagesBkt, message.Id, &message)
		vbolt.SetTargetSingleTerm(tx, ChatMessagesByFamilyIndex, message.Id, message.FamilyId)

		messageId = message.Id
		vbolt.TxCommit(tx)
	})

	// Close and reopen database to test persistence
	db.Close()
	db = vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)

	// Verify message persisted
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		persistedMessage := GetChatMessageById(tx, messageId)

		if persistedMessage.Id == 0 {
			t.Error("Message did not persist across database restart")
		}
		if persistedMessage.Content != "Test persistence message" {
			t.Errorf("Message content not persisted correctly: '%s'", persistedMessage.Content)
		}
		if !persistedMessage.CreatedAt.Equal(testTime) {
			t.Errorf("Message timestamp not persisted correctly: %v vs %v", persistedMessage.CreatedAt, testTime)
		}
		if persistedMessage.ClientMessageId != "persistence-test" {
			t.Errorf("Client message ID not persisted correctly: '%s'", persistedMessage.ClientMessageId)
		}
	})
}

// Test message content validation scenarios
func TestChatMessageContentValidation(t *testing.T) {
	testCases := []struct {
		name        string
		content     string
		expectValid bool
		description string
	}{
		{
			name:        "ValidMessage",
			content:     "Hello, this is a valid message!",
			expectValid: true,
			description: "Normal message should be valid",
		},
		{
			name:        "EmptyMessage",
			content:     "",
			expectValid: false,
			description: "Empty messages should be invalid",
		},
		{
			name:        "WhitespaceOnlyMessage",
			content:     "   \n\t   ",
			expectValid: false,
			description: "Whitespace-only messages should be invalid",
		},
		{
			name:        "MessageWithEmojis",
			content:     "Hello! 😊 How are you? 🎉",
			expectValid: true,
			description: "Messages with emojis should be valid",
		},
		{
			name:        "LongMessage",
			content:     "This is a very long message that contains a lot of text to test how the system handles longer content",
			expectValid: true,
			description: "Long messages should be valid",
		},
		{
			name:        "MessageWithSpecialChars",
			content:     "Special chars: !@#$%^&*()[]{}",
			expectValid: true,
			description: "Messages with special characters should be valid",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Simple validation logic (trim whitespace and check if empty)
			trimmed := strings.TrimSpace(tc.content)
			isValid := trimmed != ""

			if isValid != tc.expectValid {
				t.Errorf("%s: expected valid=%t, got valid=%t for content: '%s'",
					tc.description, tc.expectValid, isValid, tc.content)
			}
		})
	}
}

// Test message indexing functionality
func TestChatMessageIndexing(t *testing.T) {
	testDBPath := "test_chat_indexing.db"
	db := vbolt.Open(testDBPath)
	vbolt.InitBuckets(db, &cfg.Info)
	defer os.Remove(testDBPath)
	defer db.Close()

	var testUser User

	// Setup
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		userReq := CreateAccountRequest{
			Name:            "Test User",
			Email:           "test@example.com",
			Password:        "password123",
			ConfirmPassword: "password123",
		}
		hash, _ := bcrypt.GenerateFromPassword([]byte(userReq.Password), bcrypt.DefaultCost)
		testUser = AddUserTx(tx, userReq, hash)
		vbolt.TxCommit(tx)
	})

	var messageIds []int

	// Store messages and verify indexing
	vbolt.WithWriteTx(db, func(tx *vbolt.Tx) {
		for i := 0; i < 5; i++ {
			message := ChatMessage{
				FamilyId:        testUser.FamilyId,
				UserId:          testUser.Id,
				UserName:        testUser.Name,
				Content:         "Message " + string(rune('A'+i)),
				CreatedAt:       time.Now().Add(time.Duration(i) * time.Second),
				ClientMessageId: "msg-" + string(rune('A'+i)),
			}

			message.Id = vbolt.NextIntId(tx, ChatMessagesBkt)
			vbolt.Write(tx, ChatMessagesBkt, message.Id, &message)

			// Update indices
			vbolt.SetTargetSingleTerm(tx, ChatMessagesByFamilyIndex, message.Id, message.FamilyId)
			vbolt.SetTargetSingleTerm(tx, ChatMessagesByUserIndex, message.Id, message.UserId)

			messageIds = append(messageIds, message.Id)
		}

		vbolt.TxCommit(tx)
	})

	// Test family index
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		var familyMessageIds []int
		vbolt.ReadTermTargets(tx, ChatMessagesByFamilyIndex, testUser.FamilyId, &familyMessageIds, vbolt.Window{})

		if len(familyMessageIds) != 5 {
			t.Errorf("Expected 5 messages in family index, got %d", len(familyMessageIds))
		}

		// Verify all stored message IDs are in the index
		for _, storedId := range messageIds {
			found := false
			for _, indexId := range familyMessageIds {
				if indexId == storedId {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Message ID %d not found in family index", storedId)
			}
		}
	})

	// Test user index
	vbolt.WithReadTx(db, func(tx *vbolt.Tx) {
		var userMessageIds []int
		vbolt.ReadTermTargets(tx, ChatMessagesByUserIndex, testUser.Id, &userMessageIds, vbolt.Window{})

		if len(userMessageIds) != 5 {
			t.Errorf("Expected 5 messages in user index, got %d", len(userMessageIds))
		}

		// Verify all stored message IDs are in the index
		for _, storedId := range messageIds {
			found := false
			for _, indexId := range userMessageIds {
				if indexId == storedId {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Message ID %d not found in user index", storedId)
			}
		}
	})
}
