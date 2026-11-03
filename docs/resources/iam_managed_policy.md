---
page_title: "seaweedfs_iam_managed_policy Resource - seaweedfs"
description: Manages a SeaweedFS IAM managed policy.
---

# seaweedfs_iam_managed_policy

Manages a reusable IAM policy. Updates create and select a default policy version. Policy version limits and deletion behavior depend on the IAM server.

- `name` (required): policy name; changing it replaces the resource.
- `policy` (required): JSON policy document.
- `arn`, `id` (read-only): policy ARN.

Import using the policy ARN. Review the plan after import: different JSON representations may cause an update.
