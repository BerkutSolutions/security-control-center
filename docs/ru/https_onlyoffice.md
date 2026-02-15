# HTTPS + OnlyOffice

Этот документ описывает запуск Berkut SCC с `OnlyOffice` через `nginx` на HTTPS.

## Почему у вас была ошибка

Если браузер не доверяет сертификату, `OnlyOffice` не может зарегистрировать `ServiceWorker` внутри iframe:
- `Registration failed with SecurityError ... SSL certificate error`
- далее возможны `404` по путям `Editor.bin` из cache-пайплайна.

Это не критическая ошибка backend, это проблема доверия к TLS-сертификату.

## Вариант 1: Home / localhost (Windows)

### 1. Подготовьте `.env`

Обязательные параметры:
- `BERKUT_DOCS_ONLYOFFICE_ENABLED=true`
- `BERKUT_DOCS_ONLYOFFICE_PUBLIC_URL=/office/`
- `BERKUT_DOCS_ONLYOFFICE_INTERNAL_URL=http://onlyoffice/`
- `BERKUT_DOCS_ONLYOFFICE_APP_INTERNAL_URL=http://berkut:8080`
- `BERKUT_DOCS_ONLYOFFICE_JWT_SECRET=<secret>`
- `ONLYOFFICE_JWT_SECRET=<тот_же_secret>`
- `ONLYOFFICE_JWT_ENABLED=true`
- `PROXY_HTTP_PORT=80`
- `PROXY_HTTPS_PORT=443`
- `TLS_CERTS_PATH=./docker/certs`

### 2. Создайте сертификаты

Из корня проекта:

```powershell
cd C:\Users\chape\Desktop\SCC
New-Item -ItemType Directory -Path .\docker\certs -Force
docker run --rm -v "${PWD}\docker\certs:/certs" alpine:3.20 sh -lc "apk add --no-cache openssl >/dev/null && openssl req -x509 -nodes -newkey rsa:2048 -days 365 -subj '/CN=localhost' -keyout /certs/privkey.pem -out /certs/fullchain.pem"
```

Должны появиться:
- `docker/certs/fullchain.pem`
- `docker/certs/privkey.pem`

### 3. Поднимите стек

```powershell
docker compose --profile proxy down
docker compose --profile proxy up -d --build
```

### 4. Сделайте сертификат доверенным в Windows

Важно для ServiceWorker в OnlyOffice: просто "принять риск" в браузере недостаточно.

Импортируйте сертификат в `Trusted Root Certification Authorities`:
1. `Win + R` -> `certlm.msc`
2. `Trusted Root Certification Authorities` -> `Certificates`
3. `Import...` -> выберите сертификат (можно экспортировать `.crt` из `fullchain.pem`).

После импорта:
- полностью закройте браузер;
- откройте `https://localhost` заново;
- сделайте hard reload (`Ctrl+F5`).

## Вариант 1b: Home / localhost (Linux)

### 1. Подготовьте `.env`

Используйте те же параметры, что в Windows-варианте:
- `BERKUT_DOCS_ONLYOFFICE_ENABLED=true`
- `BERKUT_DOCS_ONLYOFFICE_PUBLIC_URL=/office/`
- `BERKUT_DOCS_ONLYOFFICE_INTERNAL_URL=http://onlyoffice/`
- `BERKUT_DOCS_ONLYOFFICE_APP_INTERNAL_URL=http://berkut:8080`
- `BERKUT_DOCS_ONLYOFFICE_JWT_SECRET=<secret>`
- `ONLYOFFICE_JWT_SECRET=<тот_же_secret>`
- `ONLYOFFICE_JWT_ENABLED=true`
- `PROXY_HTTP_PORT=80`
- `PROXY_HTTPS_PORT=443`
- `TLS_CERTS_PATH=./docker/certs`

### 2. Создайте сертификаты

Из корня проекта:

```bash
cd ~/path/to/SCC
mkdir -p docker/certs
docker run --rm -v "$(pwd)/docker/certs:/certs" alpine:3.20 sh -lc "apk add --no-cache openssl >/dev/null && openssl req -x509 -nodes -newkey rsa:2048 -days 365 -subj '/CN=localhost' -keyout /certs/privkey.pem -out /certs/fullchain.pem"
```

### 3. Поднимите стек

```bash
docker compose --profile proxy down
docker compose --profile proxy up -d --build
```

### 4. Доверьте сертификат системе и браузеру

`OnlyOffice` требует валидный TLS для `ServiceWorker`, поэтому сертификат нужно добавить в доверенные.

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

После этого:
- перезапустите браузер полностью;
- откройте `https://localhost`;
- выполните hard reload.

## Вариант 2: Prod

Рекомендуется использовать сертификаты от доверенного CA (например Let's Encrypt) и доменное имя.

Требования:
- `BERKUT_DOCS_ONLYOFFICE_PUBLIC_URL=/office/` (или полный URL в том же origin),
- `BERKUT_DOCS_ONLYOFFICE_INTERNAL_URL=http://onlyoffice/`,
- `BERKUT_DOCS_ONLYOFFICE_APP_INTERNAL_URL=http://berkut:8080`,
- одинаковый JWT секрет для Berkut и OnlyOffice,
- доступ пользователей к приложению только через `https://<domain>`.

## Быстрая диагностика

```powershell
docker compose --profile proxy ps
docker compose --profile proxy logs -f nginx
docker compose --profile proxy logs -f onlyoffice
docker compose --profile proxy logs -f berkut
```

Ключевые признаки:
- `nginx` должен быть `Up`, не `Restarting`.
- Не должно быть `cannot load certificate "/etc/nginx/certs/fullchain.pem"`.
- В browser devtools не должно быть `ServiceWorker ... SSL certificate error`.
