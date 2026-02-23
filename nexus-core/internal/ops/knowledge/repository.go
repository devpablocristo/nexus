package knowledge

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"nexus-core/internal/ops/knowledge/repository/models"
	knowledgedomain "nexus-core/internal/ops/knowledge/usecases/domain"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, in knowledgedomain.Document) (knowledgedomain.Document, error) {
	tagsRaw, _ := json.Marshal(in.Tags)
	row := models.Document{
		ID:        uuid.New(),
		OrgID:     in.OrgID,
		DocType:   string(in.DocType),
		Title:     in.Title,
		BodyMD:    in.BodyMD,
		TagsJSON:  datatypes.JSON(tagsRaw),
		SourceRef: in.SourceRef,
		CreatedBy: in.CreatedBy,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return knowledgedomain.Document{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) SearchDeterministic(ctx context.Context, orgID uuid.UUID, query string, docType *knowledgedomain.DocType, limit int) ([]knowledgedomain.Document, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 200 {
		limit = 200
	}
	q := r.db.WithContext(ctx).
		Where("org_id = ?", orgID).
		Order("created_at DESC").
		Limit(limit)
	if docType != nil && *docType != "" {
		q = q.Where("doc_type = ?", string(*docType))
	}
	trimmed := strings.TrimSpace(query)
	if trimmed != "" {
		like := "%" + strings.ToLower(trimmed) + "%"
		q = q.Where("LOWER(title) LIKE ? OR LOWER(body_md) LIKE ?", like, like)
	}
	var rows []models.Document
	if err := q.Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]knowledgedomain.Document, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, nil
}

func toDomain(m models.Document) knowledgedomain.Document {
	var tags []string
	_ = json.Unmarshal(m.TagsJSON, &tags)
	return knowledgedomain.Document{
		ID:        m.ID,
		OrgID:     m.OrgID,
		DocType:   knowledgedomain.DocType(m.DocType),
		Title:     m.Title,
		BodyMD:    m.BodyMD,
		Tags:      tags,
		SourceRef: m.SourceRef,
		CreatedBy: m.CreatedBy,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}
