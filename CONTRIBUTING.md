# Contributing

Contributions are welcome through GitHub issues and pull requests.

## Development setup

Install Go 1.25 or later and Terraform 1.14 or later, then run:

```shell
go mod download
make test
make build
```

Run `make generate` after changing provider schemas or examples. Generated
documentation must be committed with the schema change.

## Discovery documents

The provider bundles the official GTM v1 and v2 Discovery documents. Refresh
them with `make update-discovery`. The coverage test fails if a method is added
or removed without a corresponding resource, data source, or action.

## Live validation

The automated suite exercises API contracts, pagination, actions, and resource
lifecycles against local HTTP fixtures. Before a release, run representative
configurations from `examples/` against a dedicated GTM account using Google
Application Default Credentials.

Run live checks serially because the GTM API has a low default request quota.
Publishing, combining, moving, or deleting shared objects requires an isolated
test container.

## Pull requests

- Add tests for behavior changes.
- Keep API v1 compatibility when changing shared code.
- Update the changelog for user-visible changes.
- Run `make check` before opening the pull request.
