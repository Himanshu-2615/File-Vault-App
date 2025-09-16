package graph

import (
	"context"
	"net/http"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/handler"
	"github.com/himanshu/file-vault-app/backend/internal/repo"
)

type Deps struct {
	Repo      *repo.Repository
	GetUserID func(*http.Request) string
}

func isAdmin(ctx context.Context, d Deps, r *http.Request) bool {
	userID := d.GetUserID(r)
	if userID == "" {
		return false
	}
	u, err := d.Repo.GetUserByID(ctx, userID)
	if err != nil {
		return false
	}
	return u.Role == "admin"
}

func NewHandler(d Deps) http.Handler {
	fileType := graphql.NewObject(graphql.ObjectConfig{
		Name: "File",
		Fields: graphql.Fields{
			"id":            &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"filename":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"sizeBytes":     &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
			"mimeType":      &graphql.Field{Type: graphql.String},
			"isPublic":      &graphql.Field{Type: graphql.NewNonNull(graphql.Boolean)},
			"createdAt":     &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"publicToken":   &graphql.Field{Type: graphql.String},
			"downloadCount": &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
		},
	})

	storageStatsType := graphql.NewObject(graphql.ObjectConfig{
		Name: "StorageStats",
		Fields: graphql.Fields{
			"originalBytes": &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
			"dedupedBytes":  &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
			"savedBytes":    &graphql.Field{Type: graphql.NewNonNull(graphql.Int)},
			"savedPercent":  &graphql.Field{Type: graphql.NewNonNull(graphql.Float)},
		},
	})

	userType := graphql.NewObject(graphql.ObjectConfig{
		Name: "User",
		Fields: graphql.Fields{
			"id":        &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"email":     &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"name":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"role":      &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
			"createdAt": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
		},
	})

	query := graphql.NewObject(graphql.ObjectConfig{
		Name: "Query",
		Fields: graphql.Fields{
			"myFiles": &graphql.Field{
				Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(fileType))),
				Args: graphql.FieldConfigArgument{
					"limit":     &graphql.ArgumentConfig{Type: graphql.Int},
					"offset":    &graphql.ArgumentConfig{Type: graphql.Int},
					"nameLike":  &graphql.ArgumentConfig{Type: graphql.String},
					"mimeTypes": &graphql.ArgumentConfig{Type: graphql.NewList(graphql.String)},
					"sizeMin":   &graphql.ArgumentConfig{Type: graphql.Int},
					"sizeMax":   &graphql.ArgumentConfig{Type: graphql.Int},
					"dateFrom":  &graphql.ArgumentConfig{Type: graphql.String},
					"dateTo":    &graphql.ArgumentConfig{Type: graphql.String},
					"tags":      &graphql.ArgumentConfig{Type: graphql.NewList(graphql.String)},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					r := d.Repo
					userID := d.GetUserID(p.Context.Value(http.Request{}).(*http.Request))
					if userID == "" {
						return nil, nil
					}
					limit, _ := p.Args["limit"].(int)
					offset, _ := p.Args["offset"].(int)
					if limit == 0 {
						limit = 50
					}
					// Filters
					var nameLikePtr *string
					if v, ok := p.Args["nameLike"].(string); ok && v != "" {
						nameLikePtr = &v
					}
					var sizeMinPtr *int64
					if v, ok := p.Args["sizeMin"].(int); ok {
						vv := int64(v)
						sizeMinPtr = &vv
					}
					var sizeMaxPtr *int64
					if v, ok := p.Args["sizeMax"].(int); ok {
						vv := int64(v)
						sizeMaxPtr = &vv
					}
					var dateFromPtr *time.Time
					if v, ok := p.Args["dateFrom"].(string); ok && v != "" {
						if t, err := time.Parse(time.RFC3339, v); err == nil {
							dateFromPtr = &t
						}
					}
					var dateToPtr *time.Time
					if v, ok := p.Args["dateTo"].(string); ok && v != "" {
						if t, err := time.Parse(time.RFC3339, v); err == nil {
							dateToPtr = &t
						}
					}
					var mimeTypes []string
					if arr, ok := p.Args["mimeTypes"].([]any); ok {
						for _, x := range arr {
							if s, ok := x.(string); ok {
								mimeTypes = append(mimeTypes, s)
							}
						}
					}
					var tags []string
					if arr, ok := p.Args["tags"].([]any); ok {
						for _, x := range arr {
							if s, ok := x.(string); ok {
								tags = append(tags, s)
							}
						}
					}

					files, err := r.ListFilesFiltered(context.Background(), userID, repo.FileFilters{
						NameLike:  nameLikePtr,
						MIMETypes: mimeTypes,
						SizeMin:   sizeMinPtr,
						SizeMax:   sizeMaxPtr,
						DateFrom:  dateFromPtr,
						DateTo:    dateToPtr,
						Tags:      tags,
					}, limit, offset)
					if err != nil {
						return nil, err
					}
					// map to graphql
					var out []map[string]any
					for _, f := range files {
						token, _ := d.Repo.GetPublicTokenForFile(context.Background(), userID, f.ID)
						cnt, _ := d.Repo.CountDownloads(context.Background(), f.ID)
						out = append(out, map[string]any{
							"id":            f.ID,
							"filename":      f.Filename,
							"sizeBytes":     f.SizeBytes,
							"mimeType":      optStr(f.MIMEType),
							"isPublic":      f.IsPublic,
							"createdAt":     f.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
							"publicToken":   token,
							"downloadCount": cnt,
						})
					}
					return out, nil
				},
			},
			"myStorageStats": &graphql.Field{
				Type: graphql.NewNonNull(storageStatsType),
				Resolve: func(p graphql.ResolveParams) (any, error) {
					userID := d.GetUserID(p.Context.Value(http.Request{}).(*http.Request))
					if userID == "" {
						return nil, nil
					}
					orig, dedup, err := d.Repo.UserStorageStats(context.Background(), userID)
					if err != nil {
						return nil, err
					}
					saved := orig - dedup
					percent := 0.0
					if orig > 0 {
						percent = float64(saved) / float64(orig) * 100.0
					}
					return map[string]any{
						"originalBytes": orig,
						"dedupedBytes":  dedup,
						"savedBytes":    saved,
						"savedPercent":  percent,
					}, nil
				},
			},
			"allFiles": &graphql.Field{
				Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(fileType))),
				Args: graphql.FieldConfigArgument{
					"limit":  &graphql.ArgumentConfig{Type: graphql.Int},
					"offset": &graphql.ArgumentConfig{Type: graphql.Int},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					r := p.Context.Value(http.Request{}).(*http.Request)
					if !isAdmin(p.Context, d, r) {
						return nil, nil
					}
					limit, _ := p.Args["limit"].(int)
					offset, _ := p.Args["offset"].(int)
					if limit == 0 {
						limit = 50
					}
					files, err := d.Repo.ListAllFiles(context.Background(), limit, offset)
					if err != nil {
						return nil, err
					}
					var out []map[string]any
					for _, f := range files {
						token, _ := d.Repo.GetPublicTokenForFile(context.Background(), f.OwnerID, f.ID)
						cnt, _ := d.Repo.CountDownloads(context.Background(), f.ID)
						out = append(out, map[string]any{
							"id":            f.ID,
							"filename":      f.Filename,
							"sizeBytes":     f.SizeBytes,
							"mimeType":      optStr(f.MIMEType),
							"isPublic":      f.IsPublic,
							"createdAt":     f.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
							"publicToken":   token,
							"downloadCount": cnt,
						})
					}
					return out, nil
				},
			},
			"allUsers": &graphql.Field{
				Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(userType))),
				Args: graphql.FieldConfigArgument{
					"limit":  &graphql.ArgumentConfig{Type: graphql.Int},
					"offset": &graphql.ArgumentConfig{Type: graphql.Int},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					r := p.Context.Value(http.Request{}).(*http.Request)
					if !isAdmin(p.Context, d, r) {
						return nil, nil
					}
					limit, _ := p.Args["limit"].(int)
					offset, _ := p.Args["offset"].(int)
					if limit == 0 {
						limit = 50
					}
					users, err := d.Repo.ListAllUsers(context.Background(), limit, offset)
					if err != nil {
						return nil, err
					}
					var out []map[string]any
					for _, u := range users {
						out = append(out, map[string]any{
							"id":        u.ID,
							"email":     u.Email,
							"name":      u.Name,
							"role":      u.Role,
							"createdAt": u.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
						})
					}
					return out, nil
				},
			},
		},
	})

	mutation := graphql.NewObject(graphql.ObjectConfig{
		Name: "Mutation",
		Fields: graphql.Fields{
			"createPublicLink": &graphql.Field{
				Type: graphql.NewNonNull(graphql.String),
				Args: graphql.FieldConfigArgument{
					"fileId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					userID := d.GetUserID(p.Context.Value(http.Request{}).(*http.Request))
					if userID == "" {
						return "", nil
					}
					fileID := p.Args["fileId"].(string)
					token, err := d.Repo.GetOrCreatePublicToken(context.Background(), userID, fileID, func() (string, error) { return RandToken(24), nil })
					if err != nil {
						return "", err
					}
					return token, nil
				},
			},
			"revokePublicLink": &graphql.Field{
				Type: graphql.NewNonNull(graphql.Boolean),
				Args: graphql.FieldConfigArgument{
					"fileId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					userID := d.GetUserID(p.Context.Value(http.Request{}).(*http.Request))
					if userID == "" {
						return false, nil
					}
					fileID := p.Args["fileId"].(string)
					return true, d.Repo.RevokePublicToken(context.Background(), userID, fileID)
				},
			},
			"togglePublic": &graphql.Field{
				Type: graphql.NewNonNull(graphql.Boolean),
				Args: graphql.FieldConfigArgument{
					"fileId":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
					"isPublic": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Boolean)},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					userID := d.GetUserID(p.Context.Value(http.Request{}).(*http.Request))
					if userID == "" {
						return false, nil
					}
					fileID := p.Args["fileId"].(string)
					isPublic := p.Args["isPublic"].(bool)
					return true, d.Repo.SetFilePublic(context.Background(), userID, fileID, isPublic)
				},
			},
			"deleteFile": &graphql.Field{
				Type: graphql.NewNonNull(graphql.Boolean),
				Args: graphql.FieldConfigArgument{
					"fileId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					userID := d.GetUserID(p.Context.Value(http.Request{}).(*http.Request))
					if userID == "" {
						return false, nil
					}
					fileID := p.Args["fileId"].(string)
					return true, d.Repo.DeleteFileAndMaybeBlob(context.Background(), userID, fileID)
				},
			},
			"setUserRole": &graphql.Field{
				Type: graphql.NewNonNull(graphql.Boolean),
				Args: graphql.FieldConfigArgument{
					"userId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
					"role":   &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				},
				Resolve: func(p graphql.ResolveParams) (any, error) {
					r := p.Context.Value(http.Request{}).(*http.Request)
					if !isAdmin(p.Context, d, r) {
						return false, nil
					}
					userID := p.Args["userId"].(string)
					role := p.Args["role"].(string)
					return true, d.Repo.SetUserRole(context.Background(), userID, role)
				},
			},
		},
	})

	schema, _ := graphql.NewSchema(graphql.SchemaConfig{Query: query, Mutation: mutation})
	h := handler.New(&handler.Config{Schema: &schema, Pretty: true, GraphiQL: true})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// stash request in context for user extraction
		ctx := context.WithValue(r.Context(), http.Request{}, r)
		h.ContextHandler(ctx, w, r)
	})
}

func optStr(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}
