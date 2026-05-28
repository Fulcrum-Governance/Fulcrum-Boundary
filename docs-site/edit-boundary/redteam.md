# Edit Redteam

Edit Boundary redteam packs are fixture-only checks for proposed file mutation
risk paths. They classify and evaluate patch bytes, but do not apply patches or
mutate live projects.

```bash
boundary redteam --pack edit-secret-exfil
boundary redteam --pack edit-package-script-mutation
boundary redteam --pack edit-ci-deploy-mutation
boundary redteam --pack edit-destructive-delete
boundary redteam --pack edit-cross-scope-mutation
```

The expected safety signal for denied edit-risk fixtures is `applied=false`.

Canonical repository docs:
[docs/edit-boundary/REDTEAM.md](https://github.com/Fulcrum-Governance/Fulcrum-Boundary/blob/main/docs/edit-boundary/REDTEAM.md)
