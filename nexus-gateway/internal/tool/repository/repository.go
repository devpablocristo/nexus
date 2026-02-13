package repository

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"nexus-gateway/internal/tool/repository/models"
	"nexus-gateway/internal/tool/usecases"
	tooldomain "nexus-gateway/internal/tool/usecases/domain"
	"nexus-gateway/pkg/types"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(ctx context.Context, orgID uuid.UUID, t tooldomain.Tool) (tooldomain.Tool, error) {
	m := models.Tool{
		ID:               uuid.New(),
		OrgID:            orgID,
		Name:             t.Name,
		Kind:             string(t.Kind),
		Description:      t.Description,
		Method:           t.Method,
		URL:              t.URL,
		InputSchemaJSON:  t.InputSchemaJSON,
		OutputSchemaJSON: t.OutputSchemaJSON,
		ActionType:       string(t.ActionType),
		RiskLevel:        t.RiskLevel,
		Enabled:          t.Enabled,
	}
	if err := r.db.WithContext(ctx).Create(&m).Error; err != nil {
		if stringsContains(err.Error(), "duplicate key") {
			return tooldomain.Tool{}, types.NewHTTPError(http.StatusConflict, types.ErrCodeValidation, "tool already exists")
		}
		return tooldomain.Tool{}, err
	}
	return toDomain(m), nil
}

func (r *Repository) List(ctx context.Context, orgID uuid.UUID) ([]tooldomain.Tool, error) {
	var rows []models.Tool
	if err := r.db.WithContext(ctx).Where("org_id = ?", orgID).Order("name asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]tooldomain.Tool, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDomain(row))
	}
	return out, nil
}

func (r *Repository) GetByName(ctx context.Context, orgID uuid.UUID, name string) (tooldomain.Tool, error) {
	var row models.Tool
	err := r.db.WithContext(ctx).Where("org_id = ? AND name = ?", orgID, name).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tooldomain.Tool{}, types.NewHTTPError(http.StatusNotFound, types.ErrCodeNotFound, "tool not found")
		}
		return tooldomain.Tool{}, err
	}
	return toDomain(row), nil
}

func (r *Repository) UpdateByName(ctx context.Context, orgID uuid.UUID, name string, patch usecases.ToolPatch) (tooldomain.Tool, error) {
	var row models.Tool
	err := r.db.WithContext(ctx).Where("org_id = ? AND name = ?", orgID, name).Take(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return tooldomain.Tool{}, types.NewHTTPError(http.StatusNotFound, types.ErrCodeNotFound, "tool not found")
		}
		return tooldomain.Tool{}, err
	}

	if patch.Description != nil {
		row.Description = *patch.Description
	}
	if patch.Method != nil {
		row.Method = *patch.Method
	}
	if patch.URL != nil {
		row.URL = *patch.URL
	}
	if patch.ActionType != nil {
		row.ActionType = *patch.ActionType
	}
	if patch.RiskLevel != nil {
		row.RiskLevel = *patch.RiskLevel
	}
	if patch.Enabled != nil {
		row.Enabled = *patch.Enabled
	}
	if patch.InputSchema != nil {
		b, _ := json.Marshal(*patch.InputSchema)
		row.InputSchemaJSON = b
	}
	if patch.OutputSchema != nil {
		b, _ := json.Marshal(*patch.OutputSchema)
		row.OutputSchemaJSON = b
	}

	if err := r.db.WithContext(ctx).Save(&row).Error; err != nil {
		return tooldomain.Tool{}, err
	}
	return toDomain(row), nil
}

func toDomain(m models.Tool) tooldomain.Tool {
	return tooldomain.Tool{
		ID:               m.ID,
		OrgID:            m.OrgID,
		Name:             m.Name,
		Kind:             tooldomain.ToolKind(m.Kind),
		Description:      m.Description,
		Method:           m.Method,
		URL:              m.URL,
		InputSchemaJSON:  []byte(m.InputSchemaJSON),
		OutputSchemaJSON: []byte(m.OutputSchemaJSON),
		ActionType:       tooldomain.ActionType(m.ActionType),
		RiskLevel:        m.RiskLevel,
		Enabled:          m.Enabled,
		CreatedAt:        m.CreatedAt,
		UpdatedAt:        m.UpdatedAt,
	}
}

func stringsContains(s, sub string) bool {
	return len(sub) > 0 && (len(s) >= len(sub)) && (indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	// naive
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
