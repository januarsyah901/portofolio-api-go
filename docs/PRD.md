# PRD — portofolio-api-go

| | |
|---|---|
| Status | Draft v1 · 10 Nov 2026 |
| Owner | Januarsyah Akbar |
| Repo | `~/Downloads/projects/portofolio/portofolio-api-go` |
| Referensi | `../portofolio-api` (Express + Prisma, versi produksi saat ini) |

## 1. Latar belakang

`portofolio-api` adalah backend website portofolio (Express 4 + Prisma 5 + PostgreSQL). Backend ini sudah stabil dan dipakai produksi oleh `portofolio-web`.

Proyek ini menulis ulang API yang sama dalam **Go**, dengan dua tujuan:

1. **Portofolio Go untuk lamaran magang backend** (mis. TikTok Shop Backend Engineer Intern yang meminta Golang). Hasilnya harus menunjukkan Go yang idiomatis: `net/http`, `context`, error handling eksplisit, test, dan concurrency yang aman.
2. **Pengganti drop-in** yang bisa dipakai frontend tanpa perubahan, sekaligus memperbaiki beberapa kelemahan keamanan versi lama.

## 2. Tujuan & non-tujuan

### Tujuan
- **G1 · Paritas kontrak.** Semua endpoint, path, method, status code, bentuk JSON (camelCase), dan pesan error (Bahasa Indonesia) sama dengan versi Express. Frontend tidak perlu diubah.
- **G2 · Database sama.** Membaca/menulis skema PostgreSQL yang sama persis dengan yang dikelola Prisma (tabel `"Project"`, kolom camelCase ber-quote). Bisa diarahkan ke DB yang sudah ada.
- **G3 · Kompatibel kredensial.** Hash bcrypt lama tetap valid. JWT HS256 dengan `JWT_SECRET` yang sama saling diterima oleh kedua versi. API token `pf_...` lama tetap bekerja.
- **G4 · Kualitas.** Unit test untuk semua validator, test handler dengan `httptest`, integration test ke Postgres. `go vet` dan `golangci-lint` bersih. CI di GitHub Actions.
- **G5 · Operasional.** Satu binary statis, image Docker < 30 MB, graceful shutdown, structured logging (`log/slog`), health check.

### Non-tujuan (v1)
- Mengubah frontend `portofolio-web`.
- Mengganti Prisma sebagai pemilik migrasi produksi. Prisma tetap sumber kebenaran skema; repo ini hanya menyimpan salinan SQL untuk dev/test.
- Penyimpanan file ke object storage (S3/R2). Upload tetap ke disk lokal seperti versi lama.
- Pagination atau filter baru yang mengubah bentuk respons.

## 3. Pengguna

| Pengguna | Kebutuhan |
|---|---|
| Pengunjung website (via `portofolio-web`) | Membaca proyek, testimoni, video blog, pengalaman organisasi, dan konten situs. Publik, tanpa auth. |
| Admin (pemilik situs) | Login (password atau Google), CRUD semua konten, upload gambar/CV, kelola API token. |
| Agent / skrip otomasi | Akses via API token `pf_...` dengan scope `read` atau `write`. |
| Reviewer / recruiter | Membaca kode, README, dan test sebagai bukti kemampuan Go. |

## 4. Kebutuhan fungsional

Semua respons error berbentuk `{"message": "<teks Bahasa Indonesia>"}`. Pesan harus sama dengan versi Express (lihat kode referensi di `../portofolio-api/src`).

### 4.1 Autentikasi

Tiga level akses:

| Guard | Menerima | Dipakai di |
|---|---|---|
| `jwtOnly` | JWT admin | `/api/auth/me`, `/api/admin/tokens/*`, `/api/admin/upload/*` |
| `adminOrRead` | JWT admin, **atau** API token aktif apa pun, **hanya GET/HEAD** | semua `GET .../admin-list` |
| `adminOrWrite` | JWT admin, **atau** API token aktif ber-scope `write` | semua POST/PUT/DELETE konten |

Aturan:
- Header: `Authorization: Bearer <credential>`. Credential berawalan `pf_` diperlakukan sebagai API token; selain itu sebagai JWT.
- JWT: HS256, claim `userId` (int), kedaluwarsa 1 hari.
- API token disimpan sebagai SHA-256 hex. Token dicabut (`revokedAt` terisi) atau tidak ada → 401. Token `read` pada endpoint write → 403. Token pada method non-GET/HEAD di guard read → 403.
- `lastUsedAt` diperbarui secara asinkron, dan kegagalannya hanya dicatat di log.

| Method | Path | Auth | Keterangan |
|---|---|---|---|
| POST | `/api/auth/login` | — | Body `{username, password}`. 400 jika kosong, 401 jika salah. Respons `{token, user:{id, username}}`. |
| GET | `/api/auth/me` | jwtOnly | `{id, username}`; 404 jika user hilang. |
| POST | `/api/auth/google-verify` | — | Body `{accessToken}`. Verifikasi ke Google userinfo. Email harus ada di allowlist; user dibuat otomatis jika belum ada. Respons sama dengan login. |

### 4.2 API token (`/api/admin/tokens`, jwtOnly)
- `GET /` → daftar token tanpa hash/secret, urut `createdAt desc`. Field: `id, name, prefix, scope, createdAt, lastUsedAt, revokedAt`.
- `POST /` → body `{name, scope?}`. Nama 1–60 karakter (di-trim). Scope `read` (default) atau `write`. Maksimal **10 token aktif**. Respons 201 berisi field publik + `token` (plaintext, hanya sekali).
- `DELETE /:id` → set `revokedAt`. 404 jika tidak ada atau sudah dicabut.
- Format token: `pf_` + 32 byte acak base64url. `prefix` = 8 karakter pertama.

### 4.3 Projects (`/api/projects`)
- `GET /` (publik) → `visible = true`, urut `featured desc, order asc`.
- `GET /admin-list` (adminOrRead) → semua, urut `order asc`.
- `GET /:slug` (publik) → satu proyek visible; 404 jika tidak ada.
- `POST /` (adminOrWrite) → validasi `parseProjectInput(create)`; slug otomatis dari title jika kosong (`slugify` + `uniqueSlug` dengan suffix `-2`, `-3`, ...). Slug bentrok → 409 `Slug sudah dipakai proyek lain.`
- `PUT /reorder` (adminOrWrite) → body `{orderedIds: number[]}`; set `order = index + 1` dalam satu transaksi.
- `PUT /:id` (adminOrWrite) → partial update dengan `parseProjectInput(update)`; 404 jika tidak ada; 409 untuk slug bentrok.
- `DELETE /:id` (adminOrWrite).

Aturan validasi proyek (port 1:1 dari `src/lib/project.ts`):
- `title`, `description`: wajib saat create; teks tidak kosong; di-trim.
- `imageUrl, liveUrl, githubUrl, releaseUrl, year, role`: teks atau null; `""` → null.
- `slug`: teks atau null; di-`slugify`. Saat update, kosong/null → null.
- `category` ∈ `web, desktop, tool`. `status` ∈ `live, open-source, internal`.
- `highlights`: array string; di-trim; yang kosong dibuang.
- `featured`: boolean (wajib bertipe boolean jika ada).
- `tags`, `order`, `visible`: lenient — tipe salah diabaikan.
- `slugify`: lowercase, `[^a-z0-9]+` → `-`, trim `-` di kedua ujung.

### 4.4 Testimonials (`/api/testimonials`)
- `GET /` publik (visible, `createdAt desc`), `GET /admin-list` (adminOrRead, `createdAt desc`).
- `POST /`: `name`, `role`, `text` wajib. `company`, `avatarUrl` falsy → null. `rating` default 5, `visible` default true.
- `PUT /:id`: field yang ada menimpa; `rating` harus number, `visible` harus boolean, selain itu nilai lama dipertahankan.
- `DELETE /:id`.

### 4.5 Blog video (`/api/blog`)
- `GET /` publik (visible, `order asc`), `GET /admin-list`.
- `POST /`: `videoId` wajib. `caption` default `""`, `tags` default `[]`, `order` default 0, `visible` default true.
- `PUT /:id`, `DELETE /:id` dengan pola sama seperti testimonials.

### 4.6 Org experience (`/api/org-experience`)
- `GET /` publik dan `GET /admin-list`, urut `order asc, id asc`.
- `POST /`, `PUT /:id`, `DELETE /:id` dengan validasi `parseOrgExperienceInput` (port 1:1 dari `src/lib/orgExperience.ts`): `period`, `organization`, `role` wajib saat create; `type` ∈ `organization, event`; `order` harus integer; `visible` boolean; `logo` teks atau null.
- ID tidak valid → 400 `ID tidak valid.`

### 4.7 Site content (`/api/site-content`)
- `GET /` publik → objek `{ key: value }` dari semua baris.
- `PUT /:key` (adminOrWrite) → key ∈ `experience, skills, about, contact`, selain itu 404. Body divalidasi ketat per key (port 1:1 dari `src/lib/siteContent.ts`, termasuk batas panjang, jumlah item, URL wajib `https://`, dan format email). Field tak dikenal dibuang. Upsert. Respons `{key, value}`.
- `DELETE /:key` → hapus baris; respons pesan fallback.
- Pesan validasi memakai path, misalnya `"items[0].title" tidak boleh kosong.`

### 4.8 Upload (`/api/admin/upload`, jwtOnly, multipart)
- `POST /image` field `image`: maks 5 MB; jpg/jpeg/png/gif/webp (cek ekstensi **dan** isi file). Disimpan ke `public/assets/uploads/img-<unix-ms>-<rand><ext>`. Respons `{url: "/assets/uploads/<file>"}`.
- `POST /cv` field `cv`: maks 10 MB; harus PDF. Disimpan sebagai `public/cv.pdf`. Respons `{url: "/cv.pdf", message: "CV berhasil diperbarui."}`.

### 4.9 Lain-lain
- `GET /api/health` → `{"status":"OK","message":"Server portfolio berjalan dengan baik."}`. Tambahan v1: `GET /api/health/ready` yang melakukan ping ke DB (200 atau 503).
- Static: `/assets/uploads/*` dan `/cv.pdf` dari folder `public/`.
- Panic atau error tak tertangani → 500 `Terjadi kesalahan internal pada server.`
- Route tak dikenal → 404 `{"message":"Endpoint tidak ditemukan."}`.

## 5. Perbaikan keamanan (deviasi sengaja dari versi lama)

| # | Versi lama | Versi Go |
|---|---|---|
| S1 | `JWT_SECRET` fallback ke string default | Server **gagal start** jika `JWT_SECRET` kosong atau < 32 karakter. |
| S2 | CORS `origin: true` + credentials (semua origin diterima) | Allowlist dari `CORS_ORIGINS` (dipisah koma). Default dev: `http://localhost:5173`. |
| S3 | Allowlist email Google di-hardcode | Dari env `ADMIN_EMAILS`. |
| S4 | Access token Google dikirim lewat query string | Dikirim lewat header `Authorization: Bearer`. Timeout 5 detik. |
| S5 | Seed membuat `admin/admin123` | Tidak ada password default. CLI `cmd/admin create-user` meminta password, dan memberi peringatan jika < 12 karakter. |
| S6 | Tidak ada rate limit login | Rate limit per IP pada `/api/auth/login` dan `/api/auth/google-verify` (token bucket, 5 req/menit, burst 5) → 429 `Terlalu banyak percobaan. Coba lagi nanti.` |
| S7 | Body JSON tanpa batas | Maks 1 MB untuk JSON. |
| S8 | Validasi upload hanya dari mimetype header | Tambah `http.DetectContentType` pada 512 byte pertama. |
| S9 | `reorder` menerima ID apa saja | ID harus integer positif; jika ada ID tak dikenal → 400, transaksi di-rollback. |
| S10 | Pembanding password tidak constant-time ketika user tidak ada | Tetap jalankan bcrypt terhadap hash dummy saat user tidak ditemukan. |

Semua deviasi dicatat di README bagian "Differences from the Express version".

## 6. Kebutuhan non-fungsional

- **Performa:** p95 < 20 ms untuk `GET /api/projects` dengan 100 baris di mesin dev. Pool koneksi `pgxpool` (default 10).
- **Keandalan:** graceful shutdown 10 detik pada SIGINT/SIGTERM. Timeout server: read 15 dtk, write 30 dtk, idle 60 dtk.
- **Observabilitas:** `log/slog` JSON di produksi dan text di dev. Middleware access log (method, path, status, durasi, request id). Header `X-Request-ID`.
- **Konfigurasi (env):** `PORT` (3001), `DATABASE_URL`, `JWT_SECRET`, `CORS_ORIGINS`, `ADMIN_EMAILS`, `PUBLIC_DIR` (`./public`), `LOG_FORMAT` (`text|json`), `APP_ENV` (`development|production`).
- **Kompatibilitas:** Go 1.27. Dependensi minimal: `pgx/v5`, `golang-jwt/jwt/v5`, `x/crypto/bcrypt`. Rate limiter token bucket ditulis sendiri (tanpa `x/time/rate`). Routing memakai `net/http.ServeMux` standar (pattern method + wildcard) tanpa framework.

## 7. Arsitektur

```
cmd/
  api/main.go            # wiring: config → db pool → server → graceful shutdown
  admin/main.go          # CLI: create-user, reset-password
internal/
  config/                # load & validasi env
  db/                    # pgxpool + helper transaksi
  httpx/                 # JSON helper, error writer, middleware (log, recover, cors, request id, body limit, rate limit)
  auth/                  # JWT, bcrypt, API token (hash/generate/parse bearer), guard middleware
  project/               # validator, slug, repository, handler
  testimonial/
  blog/
  orgexp/
  sitecontent/           # validator per key, repository, handler
  upload/
  token/                 # handler + repository untuk API token
  server/                # router: mendaftarkan semua modul
migrations/              # salinan SQL Prisma (untuk dev/test saja)
docs/                    # PRD, openapi.yaml
```

Prinsip:
- Setiap modul domain berisi `validate.go` (murni, tanpa I/O), `repo.go` (SQL via pgx), `handler.go` (HTTP), dan `routes.go` yang mengekspor `Register(mux *http.ServeMux, deps Deps)`.
- Handler bergantung pada interface repository supaya bisa dites dengan fake.
- SQL ditulis langsung, tanpa ORM. Nama tabel dan kolom di-quote: `"Project"."createdAt"`.
- `updatedAt` diisi aplikasi (`now()`), karena `@updatedAt` milik Prisma tidak ada di level DB.
- JSON memakai struct dengan tag camelCase; `time.Time` diserialisasi RFC3339 (ISO, sama dengan Prisma).

## 8. Strategi test

| Level | Isi | Perintah |
|---|---|---|
| Unit | Port semua test vitest (`project`, `orgExperience`, `siteContent`, `apiToken`) ke table-driven Go test. Tambah test slug, bearer parsing, guard. | `go test ./...` |
| Handler | `httptest` dengan repo palsu: status code, bentuk JSON, pesan error, guard auth. | `go test ./...` |
| Integrasi | Postgres sungguhan (docker compose, port 55434) dengan skema dari `migrations/`. CRUD end-to-end per modul. Dilewati jika `TEST_DATABASE_URL` kosong. | `make test-integration` |
| Paritas | Skrip `scripts/parity.sh` yang memanggil endpoint publik versi Express dan Go lalu membandingkan JSON-nya. | manual |

Target coverage paket `validate`/`auth` ≥ 90%.

## 9. Deliverable & tooling
- `Makefile`: `run`, `build`, `test`, `test-integration`, `lint`, `db-up`, `db-down`.
- `Dockerfile` multi-stage (`golang:1.27` → `gcr.io/distroless/static`), user non-root.
- `docker-compose.dev.yml`: Postgres 16 di `127.0.0.1:55434` dengan `migrations/` di-mount ke `docker-entrypoint-initdb.d`.
- `.github/workflows/ci.yml`: vet, lint, unit test, integration test dengan service Postgres.
- `docs/openapi.yaml`: spesifikasi OpenAPI 3.1 semua endpoint.
- `README.md` (English, untuk recruiter): ringkasan, arsitektur, cara menjalankan, keputusan desain, perbedaan dengan versi Express.

## 10. Milestone

| Fase | Isi | Selesai jika |
|---|---|---|
| M0 · Fondasi | Scaffold, config, db, httpx, server, health, migrasi dev | `make run` jalan, `/api/health` 200 |
| M1 · Auth | JWT, bcrypt, API token, guard, `/api/auth/*`, `/api/admin/tokens`, CLI admin, rate limit | Test unit + handler lulus |
| M2 · Konten | projects, testimonials, blog, org-experience | Test unit + handler lulus |
| M3 · Site content & upload | Validator site content, upload gambar/CV, static files | Test unit + handler lulus |
| M4 · Ops & docs | Dockerfile, CI, OpenAPI, README, integration test | `make test-integration` lulus, image ter-build |
| M5 · Paritas | Uji `portofolio-web` lokal terhadap API Go | Frontend berjalan tanpa perubahan |

M1, M2, dan M3 dikerjakan paralel setelah M0, masing-masing oleh worker Orca di worktree terpisah.

## 11. Kriteria penerimaan
1. Semua endpoint di bagian 4 tersedia dengan status code dan pesan yang sama dengan versi Express.
2. `go vet ./...` bersih; `go test ./...` lulus; `make test-integration` lulus.
3. Login dengan akun admin yang ada di DB produksi (hash bcrypt lama) berhasil.
4. API token `pf_` yang dibuat versi Express diterima versi Go.
5. Server menolak start tanpa `JWT_SECRET` yang valid.
6. Image Docker < 30 MB dan berjalan sebagai non-root.

## 12. Risiko

| Risiko | Mitigasi |
|---|---|
| Perbedaan serialisasi JSON (null vs omitted, format waktu) | Tidak memakai `omitempty` pada field model; test snapshot JSON; skrip paritas. |
| Prisma menambah migrasi baru di repo lama | `migrations/` disinkronkan manual; README menjelaskan prosesnya. |
| Merge conflict antar worker paralel | Setiap worker hanya menyentuh paket miliknya; registrasi route sudah disiapkan di M0. |
| Perilaku "lenient" versi lama sulit direplikasi persis di Go yang bertipe statis | Decode body ke `map[string]json.RawMessage` lalu periksa tipe per field. |
