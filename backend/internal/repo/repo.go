package repo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	Pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository { return &Repository{Pool: pool} }

// User
type User struct {
	ID        string
	Email     string
	Name      string
	Role      string
	CreatedAt time.Time
}

func (r *Repository) UpsertUserByID(ctx context.Context, id string, email string, name string) (User, error) {
	const q = `
        INSERT INTO users (id, email, name)
        VALUES ($1, $2, $3)
        ON CONFLICT (id) DO UPDATE SET email = EXCLUDED.email, name = EXCLUDED.name
        RETURNING id, email, name, role, created_at`
	var u User
	err := r.Pool.QueryRow(ctx, q, id, email, name).Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.CreatedAt)
	return u, err
}

func (r *Repository) EnsureUserExists(ctx context.Context, id string) error {
	// Create placeholder if not exists
	_, err := r.Pool.Exec(ctx, `INSERT INTO users (id, email, name) VALUES ($1, $2, $3) ON CONFLICT (id) DO NOTHING`, id, id+"@local", "User")
	return err
}

// Blob
type Blob struct {
	Hash        string
	SizeBytes   int64
	MIMEType    *string
	StoragePath string
	RefCount    int64
	CreatedAt   time.Time
}

func (r *Repository) GetBlob(ctx context.Context, hash string) (Blob, error) {
	const q = `SELECT hash, size_bytes, mime_type, storage_path, ref_count, created_at FROM blobs WHERE hash=$1`
	var b Blob
	err := r.Pool.QueryRow(ctx, q, hash).Scan(&b.Hash, &b.SizeBytes, &b.MIMEType, &b.StoragePath, &b.RefCount, &b.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Blob{}, err
		}
		return Blob{}, err
	}
	return b, nil
}

func (r *Repository) InsertBlob(ctx context.Context, b Blob) error {
	const q = `INSERT INTO blobs (hash, size_bytes, mime_type, storage_path, ref_count) VALUES ($1,$2,$3,$4,$5) ON CONFLICT (hash) DO NOTHING`
	_, err := r.Pool.Exec(ctx, q, b.Hash, b.SizeBytes, b.MIMEType, b.StoragePath, b.RefCount)
	return err
}

func (r *Repository) IncBlobRef(ctx context.Context, hash string, delta int64) error {
	_, err := r.Pool.Exec(ctx, `UPDATE blobs SET ref_count = ref_count + $2 WHERE hash=$1`, hash, delta)
	return err
}

// File
type File struct {
	ID        string
	OwnerID   string
	BlobHash  string
	Filename  string
	SizeBytes int64
	MIMEType  *string
	IsPublic  bool
	Tags      []string
	CreatedAt time.Time
}

func (r *Repository) CreateFile(ctx context.Context, f File) (File, error) {
	const q = `
        INSERT INTO files (owner_id, blob_hash, filename, size_bytes, mime_type, is_public, tags)
        VALUES ($1,$2,$3,$4,$5,$6,$7)
        RETURNING id, owner_id, blob_hash, filename, size_bytes, mime_type, is_public, tags, created_at`
	var out File
	err := r.Pool.QueryRow(ctx, q, f.OwnerID, f.BlobHash, f.Filename, f.SizeBytes, f.MIMEType, f.IsPublic, f.Tags).Scan(
		&out.ID, &out.OwnerID, &out.BlobHash, &out.Filename, &out.SizeBytes, &out.MIMEType, &out.IsPublic, &out.Tags, &out.CreatedAt,
	)
	return out, err
}

func (r *Repository) ListFilesByOwner(ctx context.Context, ownerID string, limit int, offset int) ([]File, error) {
	rows, err := r.Pool.Query(ctx, `SELECT id, owner_id, blob_hash, filename, size_bytes, mime_type, is_public, tags, created_at FROM files WHERE owner_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, ownerID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []File
	for rows.Next() {
		var f File
		if err := rows.Scan(&f.ID, &f.OwnerID, &f.BlobHash, &f.Filename, &f.SizeBytes, &f.MIMEType, &f.IsPublic, &f.Tags, &f.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

type FileFilters struct {
	NameLike  *string
	MIMETypes []string
	SizeMin   *int64
	SizeMax   *int64
	DateFrom  *time.Time
	DateTo    *time.Time
	Tags      []string
}

func (r *Repository) ListFilesFiltered(ctx context.Context, ownerID string, filters FileFilters, limit int, offset int) ([]File, error) {
	where := []string{"owner_id = $1"}
	args := []any{ownerID}
	argn := 2
	if filters.NameLike != nil && *filters.NameLike != "" {
		where = append(where, "filename ILIKE $"+itoa(argn))
		args = append(args, "%"+*filters.NameLike+"%")
		argn++
	}
	if len(filters.MIMETypes) > 0 {
		where = append(where, "mime_type = ANY($"+itoa(argn)+")")
		args = append(args, filters.MIMETypes)
		argn++
	}
	if filters.SizeMin != nil {
		where = append(where, "size_bytes >= $"+itoa(argn))
		args = append(args, *filters.SizeMin)
		argn++
	}
	if filters.SizeMax != nil {
		where = append(where, "size_bytes <= $"+itoa(argn))
		args = append(args, *filters.SizeMax)
		argn++
	}
	if filters.DateFrom != nil {
		where = append(where, "created_at >= $"+itoa(argn))
		args = append(args, *filters.DateFrom)
		argn++
	}
	if filters.DateTo != nil {
		where = append(where, "created_at <= $"+itoa(argn))
		args = append(args, *filters.DateTo)
		argn++
	}
	if len(filters.Tags) > 0 {
		where = append(where, "tags && $"+itoa(argn))
		args = append(args, filters.Tags)
		argn++
	}
	query := "SELECT id, owner_id, blob_hash, filename, size_bytes, mime_type, is_public, tags, created_at FROM files WHERE " + strings.Join(where, " AND ") + " ORDER BY created_at DESC LIMIT $" + itoa(argn) + " OFFSET $" + itoa(argn+1)
	args = append(args, limit, offset)
	rows, err := r.Pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []File
	for rows.Next() {
		var f File
		if err := rows.Scan(&f.ID, &f.OwnerID, &f.BlobHash, &f.Filename, &f.SizeBytes, &f.MIMEType, &f.IsPublic, &f.Tags, &f.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func itoa(i int) string { return fmt.Sprintf("%d", i) }

func (r *Repository) SumUserStorage(ctx context.Context, ownerID string) (int64, error) {
	var sum int64
	if err := r.Pool.QueryRow(ctx, `SELECT COALESCE(SUM(size_bytes),0) FROM files WHERE owner_id=$1`, ownerID).Scan(&sum); err != nil {
		return 0, err
	}
	return sum, nil
}

func (r *Repository) UserStorageStats(ctx context.Context, ownerID string) (original int64, deduped int64, err error) {
	if err = r.Pool.QueryRow(ctx, `SELECT COALESCE(SUM(size_bytes),0) FROM files WHERE owner_id=$1`, ownerID).Scan(&original); err != nil {
		return
	}
	if err = r.Pool.QueryRow(ctx, `
        SELECT COALESCE(SUM(b.size_bytes),0)
        FROM blobs b
        WHERE b.hash IN (SELECT DISTINCT blob_hash FROM files WHERE owner_id=$1)
    `, ownerID).Scan(&deduped); err != nil {
		return
	}
	return
}

func (r *Repository) SetFilePublic(ctx context.Context, ownerID string, fileID string, isPublic bool) error {
	cmd, err := r.Pool.Exec(ctx, `UPDATE files SET is_public=$1 WHERE id=$2 AND owner_id=$3`, isPublic, fileID, ownerID)
	if err != nil {
		return err
	}
	if cmd.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *Repository) DeleteFileAndMaybeBlob(ctx context.Context, ownerID string, fileID string) error {
	// get blob hash
	var blob string
	err := r.Pool.QueryRow(ctx, `SELECT blob_hash FROM files WHERE id=$1 AND owner_id=$2`, fileID, ownerID).Scan(&blob)
	if err != nil {
		return err
	}
	// delete file
	if _, err := r.Pool.Exec(ctx, `DELETE FROM files WHERE id=$1 AND owner_id=$2`, fileID, ownerID); err != nil {
		return err
	}
	// decrement ref
	if _, err := r.Pool.Exec(ctx, `UPDATE blobs SET ref_count = ref_count - 1 WHERE hash=$1`, blob); err != nil {
		return err
	}
	// optional: purge blob when zero; leave to GC job for now
	return nil
}

// Sharing and downloads
func (r *Repository) GetOrCreatePublicToken(ctx context.Context, ownerID string, fileID string, gen func() (string, error)) (string, error) {
	// ensure ownership
	var exists bool
	if err := r.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM files WHERE id=$1 AND owner_id=$2)`, fileID, ownerID).Scan(&exists); err != nil {
		return "", err
	}
	if !exists {
		return "", pgx.ErrNoRows
	}
	// check existing
	var token string
	err := r.Pool.QueryRow(ctx, `SELECT public_token FROM shares WHERE file_id=$1 AND public_token IS NOT NULL LIMIT 1`, fileID).Scan(&token)
	if err == nil && token != "" {
		return token, nil
	}
	// create
	newToken, err := gen()
	if err != nil {
		return "", err
	}
	_, err = r.Pool.Exec(ctx, `INSERT INTO shares (file_id, public_token) VALUES ($1, $2)`, fileID, newToken)
	if err != nil {
		return "", err
	}
	return newToken, nil
}

func (r *Repository) RevokePublicToken(ctx context.Context, ownerID string, fileID string) error {
	// ensure ownership
	var exists bool
	if err := r.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM files WHERE id=$1 AND owner_id=$2)`, fileID, ownerID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return pgx.ErrNoRows
	}
	_, err := r.Pool.Exec(ctx, `DELETE FROM shares WHERE file_id=$1 AND public_token IS NOT NULL`, fileID)
	return err
}

type FileWithBlob struct {
	File
	BlobPath string
}

func (r *Repository) GetFileByPublicToken(ctx context.Context, token string) (FileWithBlob, error) {
	const q = `
        SELECT f.id, f.owner_id, f.blob_hash, f.filename, f.size_bytes, f.mime_type, f.is_public, f.tags, f.created_at, b.storage_path
        FROM shares s
        JOIN files f ON f.id = s.file_id
        JOIN blobs b ON b.hash = f.blob_hash
        WHERE s.public_token=$1
        LIMIT 1`
	var fw FileWithBlob
	err := r.Pool.QueryRow(ctx, q, token).Scan(&fw.ID, &fw.OwnerID, &fw.BlobHash, &fw.Filename, &fw.SizeBytes, &fw.MIMEType, &fw.IsPublic, &fw.Tags, &fw.CreatedAt, &fw.BlobPath)
	return fw, err
}

func (r *Repository) InsertDownload(ctx context.Context, fileID string, userID *string, ip string) error {
	_, err := r.Pool.Exec(ctx, `INSERT INTO downloads (file_id, user_id, ip) VALUES ($1,$2,$3)`, fileID, userID, ip)
	return err
}

func (r *Repository) CountDownloads(ctx context.Context, fileID string) (int64, error) {
	var c int64
	if err := r.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM downloads WHERE file_id=$1`, fileID).Scan(&c); err != nil {
		return 0, err
	}
	return c, nil
}

func (r *Repository) GetPublicTokenForFile(ctx context.Context, ownerID string, fileID string) (string, error) {
	// ensure ownership
	var exists bool
	if err := r.Pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM files WHERE id=$1 AND owner_id=$2)`, fileID, ownerID).Scan(&exists); err != nil {
		return "", err
	}
	if !exists {
		return "", pgx.ErrNoRows
	}
	var token string
	err := r.Pool.QueryRow(ctx, `SELECT public_token FROM shares WHERE file_id=$1 AND public_token IS NOT NULL LIMIT 1`, fileID).Scan(&token)
	if err != nil {
		return "", err
	}
	return token, nil
}

// Admin queries
func (r *Repository) ListAllFiles(ctx context.Context, limit int, offset int) ([]File, error) {
	rows, err := r.Pool.Query(ctx, `SELECT id, owner_id, blob_hash, filename, size_bytes, mime_type, is_public, tags, created_at FROM files ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []File
	for rows.Next() {
		var f File
		if err := rows.Scan(&f.ID, &f.OwnerID, &f.BlobHash, &f.Filename, &f.SizeBytes, &f.MIMEType, &f.IsPublic, &f.Tags, &f.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (r *Repository) ListAllUsers(ctx context.Context, limit int, offset int) ([]User, error) {
	rows, err := r.Pool.Query(ctx, `SELECT id, email, name, role, created_at FROM users ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (r *Repository) GetUserByID(ctx context.Context, id string) (User, error) {
	const q = `SELECT id, email, name, role, created_at FROM users WHERE id=$1`
	var u User
	err := r.Pool.QueryRow(ctx, q, id).Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.CreatedAt)
	return u, err
}

func (r *Repository) SetUserRole(ctx context.Context, userID string, role string) error {
	_, err := r.Pool.Exec(ctx, `UPDATE users SET role=$1 WHERE id=$2`, role, userID)
	return err
}
