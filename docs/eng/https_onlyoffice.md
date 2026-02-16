# HTTPS + OnlyOffice

This guide explains how to run Berkut SCC with `OnlyOffice` behind `nginx` over HTTPS.

## Why this error appears

If the browser does not trust your certificate, `OnlyOffice` cannot register `ServiceWorker` in the iframe:
- `Registration failed with SecurityError ... SSL certificate error`
- then `Editor.bin` cache requests may return `404`.

This is a TLS trust issue, not a backend logic issue.

## Home / localhost (Windows)

### 1. Prepare `.env`

Required values:
- `BERKUT_DOCS_ONLYOFFICE_ENABLED=true`
- `BERKUT_DOCS_ONLYOFFICE_PUBLIC_URL=/office/`
- `BERKUT_DOCS_ONLYOFFICE_INTERNAL_URL=http://onlyoffice/`
- `BERKUT_DOCS_ONLYOFFICE_APP_INTERNAL_URL=http://berkut:8080`
- `BERKUT_DOCS_ONLYOFFICE_JWT_SECRET=<secret>`
- `ONLYOFFICE_JWT_SECRET=<same_secret>`
- `ONLYOFFICE_JWT_ENABLED=true`
- `PROXY_HTTP_PORT=80`
- `PROXY_HTTPS_PORT=443`
- `TLS_CERTS_PATH=./docker/certs`

### 2. Generate certs

From project root:

```powershell
cd C:\Users\chape\Desktop\SCC
New-Item -ItemType Directory -Path .\docker\certs -Force
docker run --rm -v "${PWD}\docker\certs:/certs" alpine:3.20 sh -lc "apk add --no-cache openssl >/dev/null && openssl req -x509 -nodes -newkey rsa:2048 -days 365 -subj '/CN=localhost' -keyout /certs/privkey.pem -out /certs/fullchain.pem"
```

Expected files:
- `docker/certs/fullchain.pem`
- `docker/certs/privkey.pem`

### 3. Start stack

```powershell
docker compose --profile proxy down
docker compose --profile proxy up -d --build
```

### 4. Trust cert in Windows

For OnlyOffice ServiceWorker, browser warning bypass is not enough.

Import cert into `Trusted Root Certification Authorities`:
1. `Win + R` -> `certlm.msc`
2. `Trusted Root Certification Authorities` -> `Certificates`
3. `Import...` and select your local cert.

Then fully restart browser and hard reload (`Ctrl+F5`) at `https://localhost`.

## Home / localhost (Linux)

### 1. Prepare `.env`

Use the same values as in the Windows section:
- `BERKUT_DOCS_ONLYOFFICE_ENABLED=true`
- `BERKUT_DOCS_ONLYOFFICE_PUBLIC_URL=/office/`
- `BERKUT_DOCS_ONLYOFFICE_INTERNAL_URL=http://onlyoffice/`
- `BERKUT_DOCS_ONLYOFFICE_APP_INTERNAL_URL=http://berkut:8080`
- `BERKUT_DOCS_ONLYOFFICE_JWT_SECRET=<secret>`
- `ONLYOFFICE_JWT_SECRET=<same_secret>`
- `ONLYOFFICE_JWT_ENABLED=true`
- `PROXY_HTTP_PORT=80`
- `PROXY_HTTPS_PORT=443`
- `TLS_CERTS_PATH=./docker/certs`

### 2. Generate certs

From project root:

```bash
cd ~/path/to/SCC
mkdir -p docker/certs
docker run --rm -v "$(pwd)/docker/certs:/certs" alpine:3.20 sh -lc "apk add --no-cache openssl >/dev/null && openssl req -x509 -nodes -newkey rsa:2048 -days 365 -subj '/CN=localhost' -keyout /certs/privkey.pem -out /certs/fullchain.pem"
```

### 3. Start stack

```bash
docker compose --profile proxy down
docker compose --profile proxy up -d --build
```

### 4. Trust cert in OS/browser

`OnlyOffice` ServiceWorker requires trusted TLS, so add cert to trusted store.

Ubuntu / Debian:
```bash
sudo cp docker/certs/fullchain.pem /usr/local/share/ca-certificates/localhost-onlyoffice.crt
sudo update-ca-certificates
```

Fedora / RHEL:
```bash
sudo cp docker/certs/fullchain.pem /etc/pki/ca-trust/source/anchors/localhost-onlyoffice.crt
sudo update-ca-trust extract
```

Arch:
```bash
sudo trust anchor docker/certs/fullchain.pem
```

Then fully restart browser and hard reload `https://localhost`.

## Production

Use a trusted CA certificate and a real domain.

Keep:
- `BERKUT_DOCS_ONLYOFFICE_PUBLIC_URL=/office/` (or same-origin full URL),
- `BERKUT_DOCS_ONLYOFFICE_INTERNAL_URL=http://onlyoffice/`,
- `BERKUT_DOCS_ONLYOFFICE_APP_INTERNAL_URL=http://berkut:8080`,
- identical JWT secret for Berkut and OnlyOffice.

## Quick diagnostics

```powershell
docker compose --profile proxy ps
docker compose --profile proxy logs -f nginx
docker compose --profile proxy logs -f onlyoffice
docker compose --profile proxy logs -f berkut
```

Expected:
- `nginx` must be `Up`, not `Restarting`.
- no `cannot load certificate "/etc/nginx/certs/fullchain.pem"`.
- no `ServiceWorker ... SSL certificate error` in browser devtools.

## SCC Save Policy (DOCX)

- SCC creates a new DOCX version only through modal `Save` in the document panel.
- Change reason is mandatory for SCC version creation.
- In-editor OnlyOffice save / `Ctrl+S` does not create SCC versions.
- View/Edit switches use a fresh OnlyOffice session key per open to avoid stale session state.
