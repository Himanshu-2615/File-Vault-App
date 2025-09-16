package httpext

import (
    "net"
    "net/http"
    "os"
    "path/filepath"

    "github.com/go-chi/chi/v5"
    "github.com/himanshu/file-vault-app/backend/internal/repo"
)

type PublicDeps struct {
    Repo *repo.Repository
}

func RegisterPublicRoutes(r chi.Router, d PublicDeps) {
    r.Get("/d/{token}", func(w http.ResponseWriter, r *http.Request) {
        token := chi.URLParam(r, "token")
        fw, err := d.Repo.GetFileByPublicToken(r.Context(), token)
        if err != nil { http.NotFound(w, r); return }
        // increment downloads
        ip, _, _ := net.SplitHostPort(r.RemoteAddr)
        _ = d.Repo.InsertDownload(r.Context(), fw.ID, nil, ip)
        // stream file
        f, err := os.Open(fw.BlobPath)
        if err != nil { http.Error(w, "file missing", http.StatusNotFound); return }
        defer f.Close()
        if fw.MIMEType != nil { w.Header().Set("Content-Type", *fw.MIMEType) }
        w.Header().Set("Content-Disposition", "inline; filename=\""+filepath.Base(fw.Filename)+"\"")
        http.ServeContent(w, r, fw.Filename, fw.CreatedAt, f)
    })
}



