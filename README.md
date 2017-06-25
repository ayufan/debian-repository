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

### Access

The address of your repositories are:
* https://my-domain.com/orgs/my-org -> organization-wide repository
* https://my-domain.com/my-org/my-repo -> project-only repository

### Evict cache

You can force to evict in-memory request and package cache:
* https://my-domain.com/settings/cache/clear

This is useful to be set as Webhook for project or organization.

### Author/License

MIT, 2017, Kamil Trzciński
