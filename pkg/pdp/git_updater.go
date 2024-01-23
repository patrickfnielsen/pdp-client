package pdp

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"log/slog"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/util"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/go-git/go-git/v5/storage/memory"
)

func NewPolicyUpdater(repository string, repositoryKey string, repositoryBranch string, eventHandler func(context.Context, []PolicyBundle)) *PolicyUpdater {
	return &PolicyUpdater{
		project: PolicyProject{
			Url:           repository,
			SSHKey:        []byte(repositoryKey),
			Branch:        repositoryBranch,
			Hash:          "",
			PolicyBundles: make([]PolicyBundle, 1),
		},
		eventHandlerFunc: eventHandler,
	}
}

func (b *PolicyUpdater) Start(ctx context.Context) {
	// create bundles
	bundleUpdateTicker := time.NewTicker(time.Minute * 1)

	defer bundleUpdateTicker.Stop()
	for range bundleUpdateTicker.C {
		if ctx.Err() != nil {
			return
		}

		err := b.RunUpdate(ctx)
		if err != nil {
			continue
		}
	}
}

func (b *PolicyUpdater) RunUpdate(ctx context.Context) error {
	if ctx.Err() != nil {
		return nil
	}

	update, err := b.CheckForUpdates()
	if err != nil {
		slog.Error("failed to check for project updates", slog.String("error", err.Error()), slog.String("repo", b.project.Url))
		return err
	}

	slog.Debug(
		"Checking for project updates",
		slog.String("repo", b.project.Url),
		slog.Bool("update_avaliable", update.Available),
		slog.String("new_hash", update.NewHash),
		slog.String("old_hash", update.OldHash),
	)

	if update.Available {
		bundles, err := b.GenerateBundles()
		if err != nil {
			slog.Error("failed to create policy bundles", slog.String("error", err.Error()), slog.String("repo", b.project.Url))
			return err
		}

		b.project.Hash = update.NewHash
		b.project.PolicyBundles = bundles
		b.eventHandlerFunc(ctx, bundles)
	}

	return nil
}

func (b *PolicyUpdater) GenerateBundles() ([]PolicyBundle, error) {
	repo, err := b.getGitRepo()
	if err != nil {
		return nil, err
	}

	wt, err := repo.Worktree()
	if err != nil {
		return nil, errors.Join(err, errors.New("failed to get worktree"))
	}

	var bundles []PolicyBundle
	err = util.Walk(wt.Filesystem, "", func(fileName string, fi os.FileInfo, err error) error {
		if !fi.Mode().IsRegular() || err != nil {
			return nil
		}

		// if not a dir, write file content
		if fi.IsDir() {
			return nil
		}

		// handle policy files only
		if !strings.HasSuffix(fileName, ".rego") {
			return nil
		}

		file, err := wt.Filesystem.Open(fileName)
		if err != nil {
			return errors.Join(err, errors.New("failed to open file"))
		}

		path := strings.Replace(fileName, ".rego", "", 1)
		bundle := PolicyBundle{
			Name: path,
			Data: make([]byte, fi.Size()),
		}

		if _, err := file.Read(bundle.Data); err != nil {
			return errors.Join(err, errors.New("failed to read policy"))
		}

		bundles = append(bundles, bundle)
		return nil
	})

	if err != nil {
		return nil, errors.Join(err, errors.New("failed to walk fs"))
	}

	return bundles, nil
}

func (b *PolicyUpdater) CheckForUpdates() (*PolicyProjectUpdate, error) {
	update := PolicyProjectUpdate{
		Available: false,
		OldHash:   b.project.Hash,
		NewHash:   b.project.Hash,
	}
	head, err := b.getGitRemoteHead()
	if err != nil {
		return &update, err
	}

	if b.project.Hash != head {
		update.NewHash = head
		update.Available = true
		return &update, nil
	}

	return &update, nil
}

func (b *PolicyUpdater) getGitRemoteHead() (string, error) {
	authKey, err := b.getAuthKey()
	if b.project.SSHKey != nil && err != nil {
		return "", err
	}

	remote := git.NewRemote(memory.NewStorage(), &config.RemoteConfig{
		Name: b.project.Url,
		URLs: []string{b.project.Url},
	})

	list, err := remote.List(&git.ListOptions{
		Auth: authKey,
	})
	if err != nil {
		return "", errors.Join(err, errors.New("failed to list remote"))
	}

	for _, commit := range list {
		if !commit.Hash().IsZero() {
			return commit.Hash().String(), nil
		}
	}

	return "", errors.New("failed to find non zero commit")
}

func (b *PolicyUpdater) getGitRepo() (*git.Repository, error) {
	authKey, err := b.getAuthKey()
	if b.project.SSHKey != nil && err != nil {
		return nil, err
	}

	repo, err := git.Clone(memory.NewStorage(), memfs.New(), &git.CloneOptions{
		URL:           b.project.Url,
		ReferenceName: plumbing.NewBranchReferenceName(b.project.Branch),
		Auth:          authKey,
		SingleBranch:  true,
		Depth:         1,
	})
	if err != nil {
		return nil, errors.Join(err, errors.New("failed to clone"))
	}

	return repo, nil
}

func (b *PolicyUpdater) getAuthKey() (*ssh.PublicKeys, error) {
	authKey, err := ssh.NewPublicKeys("git", b.project.SSHKey, "")
	if err != nil {
		return nil, errors.Join(err, errors.New("failed to get authkey"))
	}

	return authKey, nil
}
