---
title: "Git"
description: "Git commands for branching, merging, rebasing, stashing, history inspection, and undoing changes."
icon: "🌿"
weight: 11
count: 50
tags: ["git", "version-control", "devops"]
---

## Setup & Config

```bash
git config --global user.name "Your Name"
git config --global user.email "you@example.com"
git config --global core.editor vim
git config --global pull.rebase true
git config --list
git config --list --global
```

## Init & Clone

```bash
git init
git init <directory>
git clone <url>
git clone <url> <directory>
git clone --depth 1 <url>                  # shallow clone
git clone --branch <branch> <url>
```

## Basic Workflow

```bash
git status
git add <file>
git add .
git add -p                                 # interactive hunk staging
git commit -m "message"
git commit --amend --no-edit              # amend last commit (staged changes)
git commit --amend -m "new message"
git push
git push -u origin <branch>
git push --force-with-lease               # safer force push
git pull
git pull --rebase
git fetch
git fetch --all --prune
```

## Branching

```bash
git branch
git branch -a                             # including remote
git branch <name>
git checkout <branch>
git checkout -b <branch>                  # create and switch
git switch <branch>
git switch -c <branch>                    # create and switch (modern)
git branch -d <branch>                    # delete (safe)
git branch -D <branch>                    # delete (force)
git push origin --delete <branch>
git branch -m <old> <new>                 # rename
```

## Merge & Rebase

```bash
git merge <branch>
git merge --no-ff <branch>               # preserve merge commit
git merge --squash <branch>
git merge --abort
git rebase main
git rebase -i HEAD~3                      # interactive rebase last 3
git rebase --abort
git rebase --continue
git cherry-pick <commit>
git cherry-pick <commit1>..<commit2>
```

## Stash

```bash
git stash
git stash push -m "description"
git stash list
git stash pop
git stash apply stash@{0}
git stash drop stash@{0}
git stash clear
git stash show -p stash@{0}
git stash branch <branch> stash@{0}
```

## Log & History

```bash
git log
git log --oneline
git log --oneline --graph --all
git log --oneline -10
git log --author="Name"
git log --since="2 weeks ago"
git log --grep="keyword"
git log -S "search_string"                # pickaxe — finds change in string
git log -- <file>
git log -p <file>                         # full diff per commit
git shortlog -sn                          # commit count per author
git blame <file>
git blame -L 10,20 <file>
git show <commit>
git show <commit>:<file>
git diff
git diff --staged
git diff <branch1>..<branch2>
git diff HEAD~1 HEAD
```

## Undo & Reset

```bash
git restore <file>                        # discard working dir changes
git restore --staged <file>               # unstage
git reset HEAD~1                          # undo last commit, keep changes staged
git reset --soft HEAD~1                  # undo commit, keep changes staged
git reset --mixed HEAD~1                 # undo commit, unstage changes
git reset --hard HEAD~1                  # undo commit and discard changes
git revert <commit>                       # safe undo via new commit
git clean -fd                             # remove untracked files and dirs
git clean -n                              # dry run
```

## Tags

```bash
git tag
git tag v1.0.0
git tag -a v1.0.0 -m "Release v1.0.0"
git tag -a v1.0.0 <commit>
git push origin v1.0.0
git push origin --tags
git tag -d v1.0.0
git push origin --delete v1.0.0
```

## Remote

```bash
git remote -v
git remote add origin <url>
git remote set-url origin <url>
git remote rename origin upstream
git remote remove <name>
git ls-remote origin
```
