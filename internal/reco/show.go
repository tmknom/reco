// Package reco は、reco の各操作の処理の流れを持つ。
package reco

import (
	"context"
	"fmt"
	"io"

	"github.com/tmknom/reco/internal/github"
	"github.com/tmknom/reco/internal/repository"
)

// ShowConfig は Show の入力。
type ShowConfig struct {
	Token string
	// Target は対象リポジトリのリポジトリ指定で、形式を検証済みであること。
	Target        repository.Reference
	Out           io.Writer
	GitHubOptions []github.Option
}

// Show は対象リポジトリを観測し、観測結果を Out に書く。
func Show(ctx context.Context, cfg ShowConfig) error {
	client := github.New(cfg.Token, cfg.GitHubOptions...)
	// 失敗の文言は対象名を含むため、ここでは包み直さない
	observed, err := client.FetchRepository(ctx, cfg.Target.Owner, cfg.Target.Repo)
	if err != nil {
		return err
	}
	return writeObservation(cfg.Out, cfg.Target, observed)
}

// writeObservation は観測結果を CLI 仕様の show の形式で書く。
// Repository 行には GitHub が返す名前ではなく、利用者が指定したリポジトリ指定を出す。
// 観測操作の失敗の文言と同じ名前で対象を示すためで、リダイレクトに追従しないため両者の差は大文字小文字に限られる。
func writeObservation(w io.Writer, target repository.Reference, observed github.Repository) error {
	_, err := fmt.Fprintf(w, "Repository: %s\n\nGeneral:\n  allow_auto_merge: %t\n", target, observed.General.AllowAutoMerge)
	return err
}
