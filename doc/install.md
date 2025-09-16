# Installation

Create the `config.yaml` file base on the [example in this repository](config.yaml). See the [configuration reference](doc/configuration.md) for more information on options. If the `-c` option is not provided, `corsair` will look for the config file in `/etc/corsair/`.

### Using Docker

```console
docker run -d \
  --name corsair \
  -p 8080:8080 \
  --mount type=bind,source="/path/to/config/",target=/etc/corsair \
  --restart=unless-stopped \
  b4bz/corsair:latest
```

Use `-e` to configure env vars if used in configuration.

### Using docker compose (recommended)

```yaml
services:
  corsair:
    image: b4bz/corsair:latest
    volumes:
      - /path/to/config/:/etc/corsair
    ports:
      - 8080:8080
    # Optionnal: Configure env vars if used in configuration:
    environment: 
      - MY_ENV-ENV=my-value 
    restart: unless-stopped
```

```console
docker compose up -d
```

### Build

Build the server.

```bash
git clone https://github.com/bastienwirtz/corsair
cd corsair
go build -o corsair ./cmd
```

Start the server

```bash
# Optionnal: Configure env vars if used in configuration:
export MY_ENV="my-value"
./corsair [-c config.yaml]    # Uses custom config file
```
