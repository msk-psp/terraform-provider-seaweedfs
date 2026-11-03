# terraform-provider-seaweedfs

Custom Terraform provider for SeaweedFS IAM operations.

## Current scope

- `seaweedfs_bucket`
  - Create via S3 `PUT /{bucket}`
  - Read via S3 `HEAD /{bucket}`
  - Manage tags via S3 `GET/PUT/DELETE /{bucket}?tagging`
  - Delete via S3 `DELETE /{bucket}`
- `seaweedfs_iam_user`
  - Create via `CreateUser`
  - Read via `GetUser`
  - Delete via `DeleteUser`
- `seaweedfs_iam_access_key`
  - Create via `CreateAccessKey`
  - Read via `ListAccessKeys`
  - Delete via `DeleteAccessKey`
- `seaweedfs_iam_user_policy`
  - Create/Update via `PutUserPolicy`
  - Read via `GetUserPolicy`
  - Delete via `DeleteUserPolicy`
- `seaweedfs_iam_group`: manage a group; deletion is refused while it has members or policies.
- `seaweedfs_iam_managed_policy`: manage a reusable policy and its default version.
- `seaweedfs_iam_group_membership`: manage one group/user membership.
- `seaweedfs_iam_group_policy_attachment`: attach one managed policy to a group.

Group resources require SeaweedFS IAM group and managed policy APIs. Existing groups, policies, memberships, attachments, and inline user policies must be imported before managing them. Import does not migrate or remove existing grants. API availability and effective permissions should be verified for your SeaweedFS version before applying changes.

## Group policy example

```hcl
resource "seaweedfs_iam_group" "readers" {
  name = "readers"
}

resource "seaweedfs_iam_managed_policy" "read" {
  name = "read-example"
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{ Effect = "Allow", Action = ["s3:GetObject"], Resource = ["arn:aws:s3:::example/*"] }]
  })
}

resource "seaweedfs_iam_group_policy_attachment" "read" {
  group_name = seaweedfs_iam_group.readers.name
  policy_arn = seaweedfs_iam_managed_policy.read.arn
}

resource "seaweedfs_iam_group_membership" "alice" {
  group_name = seaweedfs_iam_group.readers.name
  user_name  = "alice" # An existing IAM user
}
```

Import IDs: group `name`; managed policy `arn`; membership `group_name/user_name`; group policy attachment `group_name|policy_arn`; inline user policy `user_name/policy_name`. For example, `terraform import seaweedfs_iam_group_membership.alice readers/alice`. Memberships and attachments that already exist are not implicitly adopted. Use quotes around import IDs containing `|` in a shell. Review the plan after import: differing JSON representations can result in an update even when the policies are equivalent.

## Observed SeaweedFS behavior

- In live tests against a SeaweedFS S3 endpoint, user, user policy, and bucket CRUD worked.
- `CreateAccessKey` can return `ServiceFailure: Internal server error` in some SeaweedFS deployments.

## Build

```bash
go mod tidy
go build -o terraform-provider-seaweedfs
```

## Generate Docs

```bash
make docs
```

This runs `tfplugindocs` via `go generate` and updates files under `docs/`.

## Use locally with Terraform

Create `~/.terraformrc`:

```hcl
provider_installation {
  dev_overrides {
    "jonaskop/seaweedfs" = "/absolute/path/to/terraform-provider-seaweedfs"
  }
  direct {}
}
```

Then in your Terraform config:

```hcl
terraform {
  required_providers {
    seaweedfs = {
      source = "jonaskop/seaweedfs"
    }
  }
}

provider "seaweedfs" {
  endpoint   = "https://s3.example.com"
  region     = "us-east-1"
  access_key = var.admin_access_key
  secret_key = var.admin_secret_key
}

resource "seaweedfs_iam_user" "example" {
  name = "example-user"
}
```

## CI and Release

- CI workflow: `.github/workflows/ci.yml`
- Release workflow: `.github/workflows/release.yml`
- GoReleaser config: `.goreleaser.yml`

### Required GitHub Secrets for Release

- `GPG_PRIVATE_KEY`: ASCII-armored private key used to sign checksum files.
- `GPG_PASSPHRASE`: Passphrase for that key.
- `GITHUB_TOKEN` is provided automatically by GitHub Actions.

### Create First Release

```bash
git add .
git commit -m "Initial release setup"
git tag v0.1.0
git push origin main --tags
```

The release workflow will build archives and publish a GitHub release with checksums and signatures.

## Publish To Terraform Registry

1. Keep repo public and named `terraform-provider-seaweedfs`.
2. Ensure your provider address matches your namespace:
  `registry.terraform.io/jonaskop/seaweedfs`.
3. Push a semver tag (`v0.1.0`, `v0.1.1`, ...), which triggers the release workflow.
4. In Terraform Registry, publish/add the provider and link the GitHub repo `JonasKop/terraform-provider-seaweedfs`.
5. In Terraform Registry namespace settings, add your public GPG key.
6. After ingestion, users can install with:

```hcl
terraform {
  required_providers {
    seaweedfs = {
      source  = "jonaskop/seaweedfs"
      version = ">= 0.1.0"
    }
  }
}
```
