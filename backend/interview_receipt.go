package backend

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"family/cfg"
	"fmt"
	"go.hasen.dev/vbolt"
	"go.hasen.dev/vpack"
)

type InterviewReceipt struct {
	Key         string
	FamilyId    int
	Fingerprint string
	Ids         []int
}

func PackInterviewReceipt(self *InterviewReceipt, buf *vpack.Buffer) {
	vpack.Version(1, buf)
	vpack.String(&self.Key, buf)
	vpack.Int(&self.FamilyId, buf)
	vpack.String(&self.Fingerprint, buf)
	vpack.Slice(&self.Ids, vpack.Int, buf)
}

var InterviewReceiptBkt = vbolt.Bucket(&cfg.Info, "interview_receipts", vpack.StringZ, PackInterviewReceipt)
var InterviewReceiptsByFamily = vbolt.Index(&cfg.Info, "interview_receipts_by_family", vpack.FInt, vpack.StringZ)

// Call only after checking contributor access. Receipts commit with their records.
func interviewReceipt(tx *vbolt.Tx, familyId int, operation, requestId string, payload any) (InterviewReceipt, bool, error) {
	if requestId == "" {
		return InterviewReceipt{}, false, nil
	} // old clients
	if len(requestId) > 120 {
		return InterviewReceipt{}, false, errors.New("Request ID is too long")
	}
	bytes, err := json.Marshal(payload)
	if err != nil {
		return InterviewReceipt{}, false, err
	}
	receipt := InterviewReceipt{FamilyId: familyId, Key: fmt.Sprintf("%d:%s:%s", familyId, operation, requestId), Fingerprint: fmt.Sprintf("%x", sha256.Sum256(bytes))}
	var old InterviewReceipt
	if !vbolt.Read(tx, InterviewReceiptBkt, receipt.Key, &old) {
		return receipt, false, nil
	}
	if old.Fingerprint != receipt.Fingerprint {
		return receipt, false, errors.New("This request ID was already used for different interview data")
	}
	for _, id := range old.Ids {
		if i := GetInterviewById(tx, id); i.Id == 0 || i.FamilyId != familyId {
			return receipt, false, errors.New("That saved interview was deleted; start a new request")
		}
	}
	return old, true, nil
}

func saveInterviewReceipt(tx *vbolt.Tx, receipt InterviewReceipt, ids []int) {
	if receipt.Key == "" {
		return
	}
	receipt.Ids = ids
	vbolt.Write(tx, InterviewReceiptBkt, receipt.Key, &receipt)
	vbolt.SetTargetSingleTerm(tx, InterviewReceiptsByFamily, receipt.Key, receipt.FamilyId)
}

func deleteFamilyInterviewReceipts(tx *vbolt.Tx, familyId int) {
	var keys []string
	vbolt.ReadTermTargets(tx, InterviewReceiptsByFamily, familyId, &keys, vbolt.Window{})
	for _, key := range keys {
		vbolt.Delete(tx, InterviewReceiptBkt, key)
		vbolt.DeleteTargetTerms(tx, InterviewReceiptsByFamily, key)
	}
}
