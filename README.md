## Debian Repository on top of GitHub Release

This is simple application that translates GitHub Releases to behave as secured Debian Repository.

### Use

Run docker compose (with nginx-proxy):

```
server:
  image: ayufan/debian-repository
  restart: always
  expose:
  - "5000"
  volumes:
  - "/srv/data/debian-repository/cache:/cache"
  environment:
    VIRTUAL_HOST: "my-domain.com"
    VIRTUAL_PORT: 5000
    ENABLE_HTTP: "true"
    ALLOWED_ORGS: ayufan-rock64,ayufan-pine64
    GITHUB_TOKEN: my-github-token
    GPG_KEY: |
        <--- gpg signing key, generated with: gpg --export-secret-key --armor KEY_ID --->
  mem_limit: 512M
```

### Use

Access the address of your repository: https://my-domain.com/my-org/my-repo

### Author/License

MIT, 2017, Kamil Trzciński
