package toollab

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// AutoDescriptionConfig provides the minimal overrides that only the
// service itself can know.  Everything else is inferred from OpenAPI
// and DB schema automatically.
type AutoDescriptionConfig struct {
	Purpose   string
	Domain    string
	Consumers string

	// ExtraDeps allows the service to declare external dependencies
	// that cannot be discovered from the OpenAPI/schema introspection
	// (e.g. a Redis cache, an external payment gateway, etc.).
	ExtraDeps []Dependency
}

// BuildAutoDescription constructs a ServiceDescription by parsing the
// OpenAPI document (YAML or JSON) and optionally a DB schema response.
// The caller only provides high-level overrides via AutoDescriptionConfig;
// everything else (models, endpoints, relations, fields) is extracted
// automatically.
//
// openAPIRaw may be nil if no OpenAPI document is available.
// schemaJSON should be the raw JSON output of the SchemaProvider (may be nil).
func BuildAutoDescription(
	appName string,
	openAPIRaw []byte,
	schemaJSON []byte,
	cfg AutoDescriptionConfig,
) *ServiceDescription {
	desc := &ServiceDescription{
		Purpose:      cfg.Purpose,
		Domain:       cfg.Domain,
		Consumers:    cfg.Consumers,
		Dependencies: append([]Dependency{}, cfg.ExtraDeps...),
	}

	var oaDoc *oaSpec
	if len(openAPIRaw) > 0 {
		oaDoc = parseOASpec(openAPIRaw)
	}

	if oaDoc != nil {
		if desc.Purpose == "" && oaDoc.Info.Description != "" {
			desc.Purpose = oaDoc.Info.Description
		}
		if desc.Purpose == "" && oaDoc.Info.Title != "" {
			desc.Purpose = fmt.Sprintf("API %s", oaDoc.Info.Title)
		}
		desc.EndpointDescriptions = extractEndpoints(oaDoc)
		desc.Models = append(desc.Models, extractModelsFromSchemas(oaDoc)...)
	}

	if len(schemaJSON) > 0 {
		dbModels, dbDeps := extractModelsFromDBSchema(schemaJSON)
		desc.Models = mergeModels(desc.Models, dbModels)
		desc.Dependencies = append(desc.Dependencies, dbDeps...)
	}

	if desc.Domain == "" {
		desc.Domain = inferDomainFromEndpoints(desc.EndpointDescriptions)
	}

	dedup := map[string]bool{}
	filtered := desc.Dependencies[:0]
	for _, d := range desc.Dependencies {
		key := d.Name + "|" + d.Type
		if !dedup[key] {
			dedup[key] = true
			filtered = append(filtered, d)
		}
	}
	desc.Dependencies = filtered

	return desc
}

// --- OpenAPI parsing ---

type oaSpec struct {
	Info       oaInfo                      `yaml:"info" json:"info"`
	Paths      map[string]map[string]oaOp  `yaml:"paths" json:"paths"`
	Components *oaComponents               `yaml:"components" json:"components"`
}

type oaInfo struct {
	Title       string `yaml:"title" json:"title"`
	Description string `yaml:"description" json:"description"`
	Version     string `yaml:"version" json:"version"`
}

type oaOp struct {
	OperationID string            `yaml:"operationId" json:"operationId"`
	Summary     string            `yaml:"summary" json:"summary"`
	Description string            `yaml:"description" json:"description"`
	Tags        []string          `yaml:"tags" json:"tags"`
	Security    []map[string]any  `yaml:"security" json:"security"`
	RequestBody *oaRequestBody    `yaml:"requestBody" json:"requestBody"`
	Responses   map[string]oaResp `yaml:"responses" json:"responses"`
	Parameters  []oaParam         `yaml:"parameters" json:"parameters"`
}

type oaRequestBody struct {
	Content map[string]oaMediaType `yaml:"content" json:"content"`
}

type oaMediaType struct {
	Schema *oaSchema `yaml:"schema" json:"schema"`
}

type oaResp struct {
	Description string                 `yaml:"description" json:"description"`
	Content     map[string]oaMediaType `yaml:"content" json:"content"`
}

type oaParam struct {
	Name        string    `yaml:"name" json:"name"`
	In          string    `yaml:"in" json:"in"`
	Required    bool      `yaml:"required" json:"required"`
	Description string    `yaml:"description" json:"description"`
	Schema      *oaSchema `yaml:"schema" json:"schema"`
}

type oaSchema struct {
	Ref        string               `yaml:"$ref" json:"$ref"`
	Type       string               `yaml:"type" json:"type"`
	Format     string               `yaml:"format" json:"format"`
	Properties map[string]*oaSchema `yaml:"properties" json:"properties"`
	Required   []string             `yaml:"required" json:"required"`
	Items      *oaSchema            `yaml:"items" json:"items"`
}

type oaComponents struct {
	Schemas         map[string]*oaSchema `yaml:"schemas" json:"schemas"`
	SecuritySchemes map[string]any       `yaml:"securitySchemes" json:"securitySchemes"`
}

func parseOASpec(raw []byte) *oaSpec {
	var doc oaSpec
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		if err := json.Unmarshal(raw, &doc); err != nil {
			return nil
		}
	}
	return &doc
}

func extractEndpoints(doc *oaSpec) []EndpointDescription {
	if doc == nil || doc.Paths == nil {
		return nil
	}

	methodOrder := []string{"get", "post", "put", "patch", "delete", "head", "options"}
	paths := sortedKeys(doc.Paths)
	var eps []EndpointDescription

	for _, path := range paths {
		if strings.HasPrefix(path, "/_toollab") {
			continue
		}
		ops := doc.Paths[path]
		for _, m := range methodOrder {
			op, ok := ops[m]
			if !ok {
				continue
			}
			ep := EndpointDescription{
				Method:       strings.ToUpper(m),
				Path:         path,
				Summary:      op.Summary,
				Description:  op.Description,
				Category:     categorizeFromTags(op.Tags, path),
				RequiresAuth: len(op.Security) > 0,
			}

			for _, r := range op.Responses {
				if r.Description != "" && strings.Contains(strings.ToLower(r.Description), "error") {
					// best-effort: extract error codes from response descriptions
				}
			}
			for code, r := range op.Responses {
				if len(code) == 3 && code[0] >= '4' {
					status := 0
					fmt.Sscanf(code, "%d", &status)
					if status > 0 {
						ep.ErrorCodes = append(ep.ErrorCodes, ErrorCode{
							Status:      status,
							Description: r.Description,
						})
					}
				}
			}
			sort.Slice(ep.ErrorCodes, func(i, j int) bool {
				return ep.ErrorCodes[i].Status < ep.ErrorCodes[j].Status
			})

			eps = append(eps, ep)
		}
	}
	return eps
}

func extractModelsFromSchemas(doc *oaSpec) []ModelDescription {
	if doc == nil || doc.Components == nil || doc.Components.Schemas == nil {
		return nil
	}

	names := sortedKeys(doc.Components.Schemas)
	var models []ModelDescription

	for _, name := range names {
		schema := doc.Components.Schemas[name]
		if schema == nil || schema.Type != "object" || len(schema.Properties) == 0 {
			continue
		}

		reqSet := map[string]bool{}
		for _, r := range schema.Required {
			reqSet[r] = true
		}

		var fields []FieldDescription
		var relations []Relation
		fieldNames := sortedKeys(schema.Properties)

		for _, fname := range fieldNames {
			prop := schema.Properties[fname]
			if prop == nil {
				continue
			}

			ft := schemaTypeString(prop)
			fd := FieldDescription{
				Name:     fname,
				Type:     ft,
				Required: reqSet[fname],
			}

			if isRelationField(fname, prop) {
				target := inferRelationTarget(fname, prop)
				relType := "belongs_to"
				if prop.Type == "array" {
					relType = "has_many"
				}
				relations = append(relations, Relation{
					Target: target,
					Type:   relType,
				})
			}

			fields = append(fields, fd)
		}

		models = append(models, ModelDescription{
			Name:      name,
			Fields:    fields,
			Relations: relations,
		})
	}
	return models
}

// --- DB schema parsing ---

type dbSchemaResp struct {
	Database struct {
		Type    string `json:"type"`
		Version string `json:"version"`
	} `json:"database"`
	Entities []dbEntity `json:"entities"`
}

type dbEntity struct {
	Name    string     `json:"name"`
	Table   string     `json:"table"`
	Columns []dbColumn `json:"columns"`
}

type dbColumn struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
	PK       bool   `json:"pk"`
	FK       string `json:"fk,omitempty"`
}

func extractModelsFromDBSchema(raw []byte) ([]ModelDescription, []Dependency) {
	var schema dbSchemaResp
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil, nil
	}

	var deps []Dependency
	if schema.Database.Type != "" {
		deps = append(deps, Dependency{
			Name:     schema.Database.Type,
			Type:     "database",
			Required: true,
		})
	}

	var models []ModelDescription
	for _, entity := range schema.Entities {
		m := ModelDescription{
			Name:        toModelName(entity.Name),
			Description: fmt.Sprintf("Tabla de base de datos: %s", entity.Table),
		}

		for _, col := range entity.Columns {
			fd := FieldDescription{
				Name:     col.Name,
				Type:     col.Type,
				Required: col.PK || !col.Nullable,
			}
			m.Fields = append(m.Fields, fd)

			if col.FK != "" {
				parts := strings.SplitN(col.FK, ".", 2)
				if len(parts) == 2 {
					m.Relations = append(m.Relations, Relation{
						Target: toModelName(parts[0]),
						Type:   "belongs_to",
					})
				}
			}
		}
		models = append(models, m)
	}
	return models, deps
}

// --- helpers ---

var refPattern = regexp.MustCompile(`#/components/schemas/(\w+)`)

func schemaTypeString(s *oaSchema) string {
	if s.Ref != "" {
		matches := refPattern.FindStringSubmatch(s.Ref)
		if len(matches) == 2 {
			return matches[1]
		}
		return "ref"
	}
	if s.Type == "array" && s.Items != nil {
		inner := schemaTypeString(s.Items)
		return "array<" + inner + ">"
	}
	t := s.Type
	if s.Format != "" {
		t += "(" + s.Format + ")"
	}
	return t
}

func isRelationField(name string, s *oaSchema) bool {
	if strings.HasSuffix(name, "_id") || strings.HasSuffix(name, "Id") {
		return true
	}
	if s.Ref != "" {
		return true
	}
	if s.Type == "array" && s.Items != nil && s.Items.Ref != "" {
		return true
	}
	return false
}

func inferRelationTarget(name string, s *oaSchema) string {
	if s.Ref != "" {
		m := refPattern.FindStringSubmatch(s.Ref)
		if len(m) == 2 {
			return m[1]
		}
	}
	if s.Type == "array" && s.Items != nil && s.Items.Ref != "" {
		m := refPattern.FindStringSubmatch(s.Items.Ref)
		if len(m) == 2 {
			return m[1]
		}
	}
	name = strings.TrimSuffix(name, "_id")
	name = strings.TrimSuffix(name, "Id")
	return toModelName(name)
}

func toModelName(s string) string {
	s = strings.ReplaceAll(s, "_", " ")
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, "")
}

func categorizeFromTags(tags []string, path string) string {
	for _, tag := range tags {
		tl := strings.ToLower(tag)
		switch {
		case strings.Contains(tl, "admin"):
			return "admin"
		case strings.Contains(tl, "auth"):
			return "auth"
		case strings.Contains(tl, "infra") || strings.Contains(tl, "health"):
			return "infra"
		}
	}

	pl := strings.ToLower(path)
	switch {
	case strings.Contains(pl, "admin"):
		return "admin"
	case strings.Contains(pl, "auth") || strings.Contains(pl, "login") || strings.Contains(pl, "token"):
		return "auth"
	case strings.Contains(pl, "health") || strings.Contains(pl, "ready") || strings.Contains(pl, "status"):
		return "infra"
	case strings.Contains(pl, "openapi") || strings.Contains(pl, "swagger") || strings.Contains(pl, "docs"):
		return "docs"
	}
	return "business"
}

func inferDomainFromEndpoints(eps []EndpointDescription) string {
	keywords := map[string]string{
		"user":     "Gestión de usuarios",
		"auth":     "Autenticación",
		"payment":  "Pagos",
		"order":    "Pedidos",
		"product":  "Catálogo",
		"policy":   "Políticas",
		"tool":     "Herramientas",
		"agent":    "Agentes",
		"incident": "Incidentes",
		"secret":   "Secretos",
		"event":    "Eventos",
		"gateway":  "Gateway",
	}

	found := map[string]bool{}
	for _, ep := range eps {
		pl := strings.ToLower(ep.Path)
		for kw, domain := range keywords {
			if strings.Contains(pl, kw) {
				found[domain] = true
			}
		}
	}

	if len(found) == 0 {
		return "General"
	}
	domains := make([]string, 0, len(found))
	for d := range found {
		domains = append(domains, d)
	}
	sort.Strings(domains)
	return strings.Join(domains, ", ")
}

func mergeModels(a, b []ModelDescription) []ModelDescription {
	index := map[string]int{}
	for i, m := range a {
		index[strings.ToLower(m.Name)] = i
	}
	for _, m := range b {
		key := strings.ToLower(m.Name)
		if idx, exists := index[key]; exists {
			existing := &a[idx]
			if existing.Description == "" {
				existing.Description = m.Description
			}
			fieldSet := map[string]bool{}
			for _, f := range existing.Fields {
				fieldSet[f.Name] = true
			}
			for _, f := range m.Fields {
				if !fieldSet[f.Name] {
					existing.Fields = append(existing.Fields, f)
				}
			}
			relSet := map[string]bool{}
			for _, r := range existing.Relations {
				relSet[r.Target+"|"+r.Type] = true
			}
			for _, r := range m.Relations {
				if !relSet[r.Target+"|"+r.Type] {
					existing.Relations = append(existing.Relations, r)
				}
			}
		} else {
			index[key] = len(a)
			a = append(a, m)
		}
	}
	return a
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
