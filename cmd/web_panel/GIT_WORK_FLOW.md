```
╔══════════════════════════════════════════════════════════════╗
║                 🧭 Git & GitHub Workflow Guide               ║
╚══════════════════════════════════════════════════════════════╝
```

A simple guide for all team members to follow when working with Git and GitHub to avoid merge conflicts and keep the code clean.

---

## 📋 Table of Contents

- [🪄 Basic Workflow Steps](#-basic-workflow-steps)
- [✅ Important Rules     ](#-important-rules)
- [🛠️ Troubleshooting     ](#️-troubleshooting)
- [🌟 Example Branch Flow ](#-example-branch-flow)

---

## 🪄 Basic Workflow Steps

### Workflow Diagram
```
          ┌─────────────┐
          │    main     │
          └─────┬───────┘
                │
                ▼
          ┌─────────────┐
          │  feature/   │
          │  <name>     │
          └─────┬───────┘
                │
                ▼
          ┌─────────────┐
          │   commit    │
          └─────┬───────┘
                │
                ▼
          ┌─────────────┐
          │    push     │
          └─────┬───────┘
                │
                ▼
          ┌─────────────┐
          │     PR      │
          └─────┬───────┘
                │
                ▼
          ┌─────────────┐
          │    merge    │
          └─────┬───────┘
                │
                ▼
          ┌─────────────┐
          │   deploy    │
          └─────────────┘
```

### 1. 🔄 Always pull the latest main branch
Before starting any work:
```bash
git checkout main
git pull origin main
```

### 2. 🌿 Create your own branch
Each user must **create a new branch** for their feature or fix.  
Never work directly on `main`.

```bash
git checkout -b feature/<your-feature-name>
```

**Branch Naming Examples:**

________________________________________________
| Branch Name         | User  | Feature        |
|---------------------|-------|----------------|
| `feature/login`     | User1 | Login logic    |
| `feature/register`  | User2 | Register logic |
| `feature/dashboard` | User3 | Dashboard UI   |
¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯

### 3. 💻 Work and commit in your branch
Make your changes, then:
```bash
git add .
git commit -m "Describe your changes clearly"
```

### 4. 📤 Push your branch to GitHub
```bash
git push origin feature/<your-feature-name>
```

### 5. 🔀 Create a Pull Request (PR)
On GitHub:
- Go to **Pull Requests** → **New Pull Request**
- Base branch: `main`
- Compare branch: `feature/<your-feature-name>`
- Add a clear title and description of your changes

### 6. 🔄 Update your branch if `main` changes
If another PR is merged before yours, you must update your branch:

```bash
git fetch origin
git rebase origin/main
# or
git merge origin/main
```

Then fix any conflicts (if any), test again, and push:
```bash
git add .
git rebase --continue
git push origin feature/<your-feature-name> --force-with-lease
```

### 7. ✅ Merge your PR
Once your PR is reviewed and approved, **merge it** into `main` on GitHub.

### 8. 🚀 Deploy to production
On the production server:
```bash
git checkout main
git pull origin main
# restart or redeploy your app as needed
```

---

## ✅ Important Rules

_____________________________________________________________________________________
| Icon  |               Rule             |                  Description             |
|-------|--------------------------------|------------------------------------------|
|  ❌  | Never work directly on `main`  | Always create feature branches            |
|  🌿  | 1 feature = 1 branch           | Keep branches focused on single features  |
|  🔄  | Always pull the latest `main`  | Before starting work or merging           |
|  🧩  | Resolve conflicts carefully    | Test thoroughly after resolving           |
|  🧑‍💻  | Use clear commit messages      | And PR titles for better collaboration    |
¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯

---

## 🛠️ Troubleshooting

_____________________________________________________________________________________________________________________________
|              Issue                  |                                   Solution                                          |
|-------------------------------------|-------------------------------------------------------------------------------------|
| Merge conflicts during rebase/merge | Edit conflicted files, resolve manually, then `git add` and `git rebase --continue` |
| Branch not up to date with main     | Run `git fetch origin` then `git rebase origin/main`                                |
| Push rejected (non-fast-forward)    | Update your branch first, then push with `--force-with-lease`                       |
| Forgot to pull latest main          | Always start with `git checkout main && git pull origin main`                       |
| PR has conflicts                    | Update your branch from main and resolve conflicts locally                          |
¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯¯
---

## 🌟 Example Branch Flow

```bash
# Start
git checkout main
git pull origin main

# Create your branch
git checkout -b feature/login

# Work & commit
git add .
git commit -m "Add login logic"

# Push & open PR
git push origin feature/login
```

---

**Last updated:** October 2025  
**Maintained by:** Development Team
