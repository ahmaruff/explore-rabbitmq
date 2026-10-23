# Example Go API: Arsitektur dan Aturan Tim

Status: living document. Dokumen ini menjelaskan cara menambah module, cara wiring, dan batasan yang tidak boleh dilanggar. Isinya aturan jangka panjang, bukan daftar endpoint yang ada saat ini.

## 1. Konteks

- Repo ini adalah REST API Go yang dipakai sebagai contoh dan titik awal untuk service baru.
- Entry point ada di `cmd/api`; nama Go module adalah `rabbitmq-example`.
- Stack utama: Echo v5 untuk HTTP, zerolog + timberjack untuk logging, `database/sql` dengan driver pgx untuk PostgreSQL (driver MySQL juga tersedia di `platform/db`), JWT HS256, go-playground/validator, dan abstraksi storage dengan driver local disk atau S3.
- Konfigurasi dibaca dari environment variable. `godotenv` memuat `.env` saat development.
- Beberapa komponen sudah tersedia tetapi wiring-nya masih dikomentari di `main.go`: database, JWT service, `health`, dan `auth`. Helper `openDatabase` dan `setupJWTService` sudah siap dipakai saat komponen itu diaktifkan.

## 2. Architecture style

Codebase ini memakai **hexagonal architecture**, atau **ports and adapters**. Konsepnya sama dengan onion architecture: business rule ada di tengah, dan semua hal eksternal adalah adapter di sekelilingnya.

| Konsep | Di codebase ini |
| --- | --- |
| Core / use case | `service.go` dan domain type di `domain.go` |
| Port | Interface yang dideklarasikan oleh consumer, biasanya di `domain.go` |
| Inbound adapter | `handler.go`: HTTP request masuk, response keluar |
| Outbound adapter | `repository.go`: akses database |
| Composition root | `cmd/api/main.go`: membuat adapter dan meng-inject port |

Dependency direction:

```mermaid
flowchart LR
    H[handler.go<br/>inbound adapter] --> S[service.go<br/>core]
    S --> P[interface<br/>port]
    R[repository.go<br/>outbound adapter] -. implements .-> P
    MAIN[main.go] --> H
    MAIN --> S
    MAIN --> R
```

Implikasi dari pendekatan ini:

- Service bergantung pada interface, bukan pada repository konkret, HTTP type, atau database.
- Repository mengimplementasikan interface yang dideklarasikan oleh module tempat service berada.
- Handler menerjemahkan HTTP menjadi service call. Tidak ada business rule di handler.
- `main.go` adalah satu-satunya tempat yang tahu semua concrete type.
- Core bisa di-test dengan stub karena tidak punya dependency ke infrastructure.

Project ini sengaja memakai **consumer-declared port**, bukan satu package `ports` di tengah. Module yang membutuhkan capability mendeklarasikan interface paling kecil yang dia butuhkan. Ini menjaga module tetap independen dan menghindari shared package yang di-import semua module. Module kecil boleh mendeklarasikan port di file yang memakainya; contoh `Pinger` di `internal/health/handler.go`.

## 3. Struktur module

Module adalah satu feature folder di dalam `internal/`. Satu folder adalah satu Go package. Module yang ada saat ini: `auth`, `health`, dan `hello`.

Struktur standar untuk module yang punya business rule dan persistence. Module kecil hanya memakai file yang dibutuhkannya:

```text
internal/event/
  domain.go        entity, request/input struct, port
  repository.go    SQL dan row mapping; outbound adapter
  service.go       business rule; use case
  handler.go       HTTP binding, validation, response; inbound adapter
  errors.go        sentinel error dan MapError
  route.go         RegisterRoutes
  service_test.go
  repository_test.go
  route_test.go
```

| File | Tanggung jawab | Tidak boleh berisi |
| --- | --- | --- |
| `domain.go` | Domain struct, repository interface, consumer-declared port, input/params struct | SQL, HTTP, business flow |
| `repository.go` | Query, scanning, transaction, DB error wrapping | Business rule, HTTP, logging |
| `service.go` | Business rule, permission check, text cleaning, use-case orchestration | SQL, HTTP, `echo` |
| `handler.go` | Bind, validate, baca identity, panggil service, tulis response | Business rule, SQL |
| `errors.go` | Sentinel, `HTTPError`, `MapError` | Side effect, logging |
| `route.go` | Registrasi URL, grup public/protected | Business logic |
| `*_test.go` | Service rule, repository mapping, route behavior | External service asli |

Aturan:

1. Jangan membuat subpackage untuk satu feature kecuali feature benar-benar independen. Pisahkan file berdasarkan concern. Contoh di `internal/auth`: `jwt.go`, `middleware.go`, `repository.go`, dan `service.go`.
2. Nama package harus sama dengan nama folder.
3. Feature-specific code tetap di feature. Jangan pindahkan ke `platform` hanya karena "mungkin bisa dipakai ulang".
4. Jangan import type, error, atau service dari module lain. Gunakan port. Lihat bagian 4.

## 4. Batasan antar module (Module Boundaries)

Import yang diperbolehkan:

| Dari | Boleh import |
| --- | --- |
| Feature module | package-nya sendiri, standard library, dan `internal/platform/*` |
| `internal/platform/*` | standard library dan package platform lain |
| `cmd/api` | semua module dan package platform |
| Feature A | tidak boleh import Feature B |

`internal/platform` adalah satu-satunya shared layer. Isinya infrastructure, bukan business rule.

Komunikasi antar module selalu lewat interface:

```mermaid
flowchart LR
    A[Feature A] -->|declares| P[PortA interface]
    B[Feature B] -. implements .-> P
    M[main.go] -->|injects B into A| A
    A --> PLATFORM[internal/platform]
    B --> PLATFORM
```

Contoh dari code saat ini:

- `cmd/api` membuat middleware dengan `auth.JWTMiddleware(jwtService)`, lalu mengirimnya sebagai parameter ke `RegisterRoutes`. File route menerima middleware itu tanpa perlu import `internal/auth`.
- Module yang butuh data dari module lain mendeklarasikan port kecil di `domain.go` miliknya sendiri, dan `main.go` yang menyuntikkan implementasinya.
- `platform/identity` menyimpan hasil authentication di request context, sehingga module tidak perlu tahu cara token dibuat atau divalidasi.

Kalau method set provider tidak cocok dengan port:

1. Buat adapter di `cmd/api` atau package adapter kecil.
2. Buat adapter mengimplementasikan port tersebut.
3. Inject adapter di `main.go`.

Jangan menyelesaikan mismatch dengan import provider dari consumer. Jangan memakai domain struct milik module lain sebagai shortcut. Kalau hanya butuh sebuah value, deklarasikan port kecil yang mengembalikan value tersebut.

Enforcement: code review wajib menolak import antar feature. Kalau nanti menambah linter, konfigurasikan untuk menolak import dari satu feature package ke feature lain; hanya `cmd/api` dan `platform` yang dikecualikan.

## 5. Wiring di main.go

`cmd/api/main.go` adalah composition root. Hanya file ini yang boleh tahu semua concrete implementation.

Urutan wiring yang dipakai saat ini:

```text
config -> logger -> storage -> files -> routes -> server start
```

Jika database dan authentication diaktifkan, urutannya menjadi:

```text
config -> logger -> database -> repositories -> services -> middleware -> routes -> server start
```

Bentuk wiring di `cmd/api/main.go`:

```go
cfg, err := loadConfig()

setupLogger(cfg)
defer logger.Close()

startupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
defer cancel()

server := platformhttp.NewServer(cfg.HTTPPort)

fileStore, err := storage.New(startupCtx, cfg.Storage)
defer fileStore.Close()

fileService, err := files.New(files.Config{
    Store:            fileStore,
    BaseURL:          cfg.AppURL,
    SigningKey:       cfg.Storage.SigningKey,
    RequireSignature: cfg.Storage.RequireSignature,
})

hello.RegisterRoutes(server.Echo())
server.Static(files.RoutePattern, files.RoutePrefix, fileService)
```

Bagian database dan auth sudah disiapkan tetapi masih dikomentari:

```go
database, err := openDatabase(cfg)
defer database.Close()

jwtService, err := setupJWTService(cfg)

userRepository := auth.NewUserRepository(database)
authService := auth.NewUserService(userRepository)
protected := auth.JWTMiddleware(jwtService)

health.RegisterRoutes(server.Echo(), database)
auth.RegisterRoutes(server.Echo(), protected, authService)
```

Aturan:

1. Buat dependency sebelum object yang membutuhkannya.
2. Kirim interface ke service. Concrete type tetap di `main.go`.
3. Kirim middleware ke `RegisterRoutes`; jangan import `auth` di file route milik feature.
4. Jangan pakai global database, global service, service locator, atau package-level singleton.
5. Jangan wiring dependency di `init()`.
6. Validasi config wajib saat startup (JWT secret, TTL, koneksi database) dan fail fast.
7. Daftarkan route setelah semua service siap.
8. Side effect startup tetap di `main.go`: config, logger, database, background loader.
9. Shutdown lewat `signal.NotifyContext` dan `server.Start(ctx)`; biarkan server menyelesaikan request yang sedang jalan (graceful timeout 10 detik).

## 6. Menambah module baru

Contoh: `internal/event`.

### Step 1: buat package

Buat `internal/event` dengan file dari bagian 3. Mulai dari `domain.go`.

### Step 2: definisikan core dan port

```go
package event

import (
    "context"
    "time"
)

type Event struct {
    ID        int       `json:"id"`
    UserID    string    `json:"user_id"`
    CreatedAt time.Time `json:"created_at"`
}

type Repository interface {
    FindEventByID(ctx context.Context, id int) (*Event, error)
    CreateEvent(ctx context.Context, params CreateEventParams) (*Event, error)
}

type UserStatusReader interface {
    IsActive(ctx context.Context, userID string) (bool, error)
}
```

Deklarasikan hanya method yang benar-benar dipakai module. Port bukan salinan repository dari module lain.

### Step 3: implementasikan repository adapter

- Gunakan placeholder sesuai driver: `$1` untuk PostgreSQL/pgx, `?` untuk MySQL.
- `ctx` selalu jadi argumen pertama.
- Wrap DB error dengan sentinel dan `%w`.
- Pasang stack sekali di tempat error dibuat: `logger.WithStack(...)`.
- Kembalikan sentinel milik feature untuk not found, bukan `sql.ErrNoRows`.
- Gunakan transaction kalau satu request menulis ke beberapa table.
- Jangan logging dari repository.

### Step 4: implementasikan service

- Terima port lewat constructor.
- Terapkan business rule dan kembalikan sentinel error.
- Bersihkan setiap string dari user sebelum disimpan, lalu cek empty dan length setelah cleaning. `platform/contentcleaner` menyediakan trim, sanitize HTML, linkify, dan masking email/phone.
- Jangan menulis SQL, HTTP status code, atau type dari `echo`.

### Step 5: definisikan error dan mapping

- Satu sentinel untuk satu business rule.
- Satu branch `MapError` untuk setiap sentinel.
- Status code dan message harus konsisten dengan kontrak API.
- Error level field mengembalikan `422` dengan map `errors` yang key-nya JSON field.

### Step 6: implementasikan handler

- Bind dengan `echo.BindBody`; JSON rusak adalah `400`.
- Validate dengan `c.Validate`; failure mengembalikan `422`.
- Pakai tag `notblank` yang sudah ada untuk menolak string berisi spasi saja.
- Kalau menambah validation tag baru, tambahkan case di `platform/http/validator.go`.
- Baca user ID dengan `identity.UserID(c)` atau `identity.UserIDInt(c)`. Keduanya mengembalikan `(value, bool)`; anggap `false` sebagai `401`.
- Kembalikan semua error service lewat `writeError` milik module.

### Step 7: daftarkan route

```go
func RegisterRoutes(e *echo.Echo, protected echo.MiddlewareFunc, service *Service) {
    h := NewHandler(service)

    e.GET("/events/:eventId", h.GetEvent)

    authorized := e.Group("/events", protected)
    authorized.POST("", h.CreateEvent)
}
```

### Step 8: wiring

```go
eventRepository := event.NewEventRepository(database)
eventService := event.NewService(eventRepository, userStatusReader)

event.RegisterRoutes(server.Echo(), protected, eventService)
```

Kalau provider tidak memenuhi port yang dideklarasikan `event`, buat adapter di `main.go` lalu inject adapter tersebut. Jangan import provider dari package `event`.

### Step 9: test

- Satu service test untuk setiap business rule.
- Satu repository test untuk setiap scan helper atau mapping yang tidak trivial.
- Satu route test untuk auth dan response envelope.
- Jalankan `go vet ./...`, `go test ./...`, dan `go build ./...`.

## 7. Yang disediakan platform

`internal/platform` ada supaya module tidak menulis infrastructure yang sama berulang kali. Gunakan ini. Jangan membuat logger kedua, response helper kedua, validator kedua, config loader kedua, atau database opener kedua.

| Package | Capability |
| --- | --- |
| `platform/config` | Load environment variable ke `Config` yang typed. Tambahkan variable baru di sini. |
| `platform/db` | Buka PostgreSQL lewat pgx (dan MySQL lewat go-sql-driver), atur connection pool, lalu ping saat startup. Mengembalikan `*sql.DB`. |
| `platform/logger` | Process logger, request-scoped logger, error helper, one-time stack attachment. |
| `platform/http` | Echo server, middleware stack, response envelope, request validator, central `HTTPErrorHandler`. |
| `platform/identity` | Menyimpan dan membaca user ID hasil authentication di request context. |
| `platform/contentcleaner` | Trim, HTML sanitize, linkify, dan masking email dan phone. |
| `platform/textutil` | Helper link, email, dan phone yang dipakai cleaner. |
| `platform/storage` | Abstraksi object storage (`Store`) dengan driver local disk dan S3. |
| `platform/files` | Validasi upload, pembuatan key, serving (`http.Handler`), dan signing URL. |

`internal/platform/http` punya nama package `http`, jadi import sebagai `platformhttp` supaya tidak menimpa standard library.

Semua response JSON memakai envelope yang sama. `success` bernilai `true` saat `code < 400`, dan `errors` hanya muncul pada validation failure:

```json
{
  "success": false,
  "code": 422,
  "message": "The given data was invalid.",
  "errors": {
    "content": ["The content field is required."]
  }
}
```

Error dari framework (404, 405, 413) dan panic yang di-recover juga lewat handler yang sama, jadi bentuk response tetap konsisten. Pengecualian: `platform/files` mengirim byte file lewat `http.ServeContent`, jadi memakai semantics `net/http` (Range, HEAD), bukan envelope.

`NewServer` memasang middleware dengan urutan: `RequestID`, request logger context, `Recover`, `BodyLimit` 2 MiB, lalu access log. Validator dan `HTTPErrorHandler` juga didaftarkan di sana, jadi feature tidak perlu memasangnya sendiri.

`auth`, `health`, dan `hello` adalah module, bukan platform. Depend lewat port dan inject di `main.go`.

API yang sering dipakai:

```go
platformhttp.OK(c, "OK", data)
platformhttp.Created(c, "Created", data)
platformhttp.Error(c, http.StatusForbidden, "Forbidden")
platformhttp.ValidationError(c, err, "The given data was invalid.")

logger.L()              // di luar request
logger.Ctx(ctx)         // di dalam request; sudah membawa request_id
logger.Err(ctx, err)    // log sekali di decision boundary
logger.WithStack(err)   // sekali, di tempat error dibuat

identity.UserID(c)      // (string, bool) subject dari JWT
identity.UserIDInt(c)   // (int, bool) subject numerik, kalau sub berupa angka
```

Ke mana code baru harus diletakkan:

| Code | Lokasi |
| --- | --- |
| Infrastructure yang dipakai lebih dari satu module | `internal/platform/<capability>` |
| Business rule untuk satu feature | `service.go` milik feature tersebut |
| SQL untuk satu feature | `repository.go` milik feature tersebut |
| Interface yang dibutuhkan satu feature | `domain.go` milik feature tersebut |
| Adapter antara dua feature | `cmd/api` atau package adapter kecil |

Aturan untuk `platform`:

1. Platform tidak boleh import feature module mana pun.
2. Platform tidak boleh berisi business rule, permission check, atau feature-specific type.
3. Pindahkan code ke platform hanya kalau itu infrastructure dan minimal module kedua membutuhkannya, atau jelas cross-cutting.
4. Jaga public API tetap kecil. Pilih beberapa function yang fokus, bukan package utility yang lebar.
5. Tambahkan test saat menambah capability platform.
6. Jangan membuat dumping ground bernama `common`, `helpers`, atau `utils`. Beri nama package sesuai capability.

Shared service module bukan platform. Perlakukan seperti module lain: depend melalui interface dan inject di `main.go`.

## 8. Aturan tim

### WAJIB

1. Ikuti handler -> service -> repository. Jangan skip layer.
2. Jangan masukkan HTTP dan `echo` ke service atau repository.
3. Jangan masukkan SQL ke service atau handler.
4. Semua response lewat `platformhttp`. Jangan panggil `c.JSON` langsung.
5. Bind dan validasi bentuk request di handler. Business rule yang butuh database tetap di service.
6. Deklarasikan port untuk setiap cross-module dependency. Inject implementation di `main.go`.
7. Feature tidak boleh import feature lain. Hanya `platform` yang shared.
8. Kembalikan sentinel untuk setiap business failure dan mapping di `MapError` module tersebut.
9. Pasang stack sekali di origin. Gunakan `%w` di layer atasnya.
10. Logging sekali, di layer yang menentukan outcome. Jangan logging di repository.
11. Kirim `ctx` ke setiap operasi I/O.
12. Bersihkan setiap string dari user sebelum disimpan. Cek empty lagi setelah cleaning.
13. Tegakkan uniqueness dengan database constraint plus `ON CONFLICT` atau retry. Jangan bergantung pada check-then-insert.
14. Gunakan transaction kalau satu request menulis ke beberapa table.
15. Validasi config wajib saat startup dan fail fast.
16. Tambahkan test untuk aturan baru. `go vet ./...`, `go test ./...`, dan `go build ./...` harus lulus.

### SEBAIKNYA

1. Pilih code eksplisit daripada reflection, registry, atau definisi berbasis meta.
2. Constructor mengembalikan concrete type; consumer menerima interface.
3. Gunakan response struct kalau bentuk API berbeda dari domain struct.
4. Simpan numeric limit di satu tempat dan jaga API message tetap sinkron.
5. Tambahkan environment variable baru ke `.env.example` dan README.
6. Update test yang sudah ada, jangan membuat setup test paralel.
7. Jalankan `gofmt` pada file yang diubah.

### JANGAN PERNAH

1. Jangan logging token, password, secret, atau credential mentah.
2. Jangan mengembalikan raw database error ke client.
3. Jangan memakai package-level database handle atau service.
4. Jangan menaruh business logic di handler "untuk sementara".
5. Jangan import module feature lain, bahkan untuk satu type kecil.

## 9. Docker

Jalankan dari root repository. Kedua compose file membaca `.env`, memakai container name `rabbitmq-example`, dan memetakan port `8090:8090`.

Development dengan hot reload:

```bash
docker compose -f docker-compose.dev.yml up --build
```

Production-like verification:

```bash
docker compose -f docker-compose.yml up --build
```

Perbedaan:

| File | Behavior |
| --- | --- |
| `docker-compose.dev.yml` | Memakai `Dockerfile.dev` (image Go + `air`), mount seluruh source ke `/app`, dan menjalankan `air -c .air.toml`. Perubahan code memicu rebuild. |
| `docker-compose.yml` | Memakai `Dockerfile` multi-stage: build binary di `golang:1.27-alpine`, lalu jalankan di `alpine` tanpa source. Pakai untuk memverifikasi build yang sebenarnya. |

Keduanya menulis log ke `./logs` dan menyimpan upload ke `./storage/app`, jadi data itu tetap ada saat container di-rebuild. `HTTP_PORT` di `.env` harus sinkron dengan mapping port di compose file, dan `LOG_FILE` harus tetap berada di dalam `/app/logs`.

PostgreSQL berjalan di host, sehingga `DB_HOST` harus `host.docker.internal` saat memakai Docker. Gunakan `docker logs -f rabbitmq-example` untuk mengikuti log. Rebuild dengan `--build` setelah mengubah dependency, Dockerfile, atau compose file.

## 10. Testing dan command

```bash
go vet ./...
go test ./...
go test ./internal/auth -cover
go build ./...
```

Convention:

- Service dan consumer test memakai fake manual. Contoh: `fakeUserRepository` di `auth` dan `fakePinger` di `health`. Jangan menambah mocking library tanpa kesepakatan.
- Repository test (kalau ditambahkan) mencakup row mapping dengan fake `rowScanner`; tambahkan integration test kalau query perlu verifikasi SQL asli.
- Route test memakai `echo.New()` atau `platformhttp.NewServer("0")` plus `httptest`, lalu memeriksa status code dan response envelope.
- Platform punya test sendiri untuk validator, response envelope, file serving dan signed URL, storage round trip, serta output logger. Jalankan juga saat mengubah `internal/platform`.
- Tambahkan atau update test di pull request yang sama dengan perubahan aturan.
