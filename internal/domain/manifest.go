package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"time"
)

func (s *RiggingSession) BuildManifest(now time.Time) (*FrozenManifest, error) {
	if err := s.requireStatus(StatusApproved); err != nil {
		return nil, err
	}
	if s.Review == nil || s.Review.Decision != "APPROVE" {
		return nil, NewError(ErrState, "review", "缺少有效批准")
	}
	lines := append([]RiggingLine(nil), s.Lines...)
	sort.Slice(lines, func(i, j int) bool { return lines[i].Code < lines[j].Code })
	manifest := &FrozenManifest{Version: s.Version + 1, FrozenAt: now.UTC(), Lines: []ManifestLine{}}
	for i, line := range lines {
		item := ManifestLine{Sequence: i + 1, LineCode: line.Code, TotalLoadGram: line.TotalLoadGram, Components: []ManifestComponent{}}
		for _, load := range s.Loads {
			if load.LineID == line.ID {
				pointCode := ""
				if point, err := s.FindPoint(load.PointID); err == nil {
					pointCode = point.Code
				}
				item.Components = append(item.Components, ManifestComponent{ComponentCode: load.ComponentCode, PointCode: pointCode, Description: load.Description, WeightGram: load.WeightGram, PositionMillimeter: load.PositionMillimeter, Quantity: load.Quantity})
			}
		}
		sort.Slice(item.Components, func(i, j int) bool { return item.Components[i].ComponentCode < item.Components[j].ComponentCode })
		manifest.Lines = append(manifest.Lines, item)
	}
	manifest.Digest = DigestManifest(manifest, s.RuleSetVersion, s.Review.ReviewerID)
	s.Frozen = manifest
	s.Status = StatusFrozen
	return manifest, nil
}
func DigestManifest(m *FrozenManifest, rule, approvedBy string) string {
	canonical := struct {
		Version    int64          `json:"version"`
		Rule       string         `json:"rule"`
		ApprovedBy string         `json:"approvedBy"`
		Lines      []ManifestLine `json:"lines"`
	}{m.Version, rule, approvedBy, m.Lines}
	data, _ := json.Marshal(canonical)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func (s *RiggingSession) IssueCertificate(id string, now time.Time) (*ReleaseCertificate, error) {
	if s.Status == StatusReleased && s.Certificate != nil {
		return s.Certificate, nil
	}
	if err := s.requireStatus(StatusFrozen); err != nil {
		return nil, err
	}
	if s.Frozen == nil || s.Review == nil {
		return nil, NewError(ErrState, "frozen", "冻结材料不完整")
	}
	s.Certificate = &ReleaseCertificate{ID: id, SessionID: s.ID, FrozenVersion: s.Frozen.Version, ManifestDigest: s.Frozen.Digest, RuleSetVersion: s.RuleSetVersion, ApprovedBy: s.Review.ReviewerID, IssuedAt: now.UTC(), VerificationStatus: "VALID"}
	s.Status = StatusReleased
	return s.Certificate, nil
}
func (s *RiggingSession) VerifyCertificate() bool {
	if s.Certificate == nil || s.Frozen == nil || s.Review == nil {
		return false
	}
	digest := DigestManifest(s.Frozen, s.RuleSetVersion, s.Review.ReviewerID)
	return digest == s.Frozen.Digest && digest == s.Certificate.ManifestDigest && s.Certificate.SessionID == s.ID && s.Certificate.FrozenVersion == s.Frozen.Version
}
