# Actions 許可ポリシーの切り替え

`actions.permissions.allowed_actions` を `selected` とほかの値（`all`、`local_only`）のあいだで変えると、GitHub の仕様により、`selected_actions` の値が消えたり、一時的に設定ファイルと違う値になったりします。

## `selected` から `all` や `local_only` に変えるとき

GitHub は `selected_actions` の値を消します。あとで `selected` に戻しても、消える前の値は戻りません。

消える値は、変更計画の `Side effect:` の行に出ます。

```text
Repository: example/project-a
  actions.permissions.allowed_actions: selected -> all
    Side effect: GitHub discards the current selected_actions (github_owned_allowed: true, verified_allowed: true, patterns_allowed: ["example/*"])
```

あとで同じ値に戻す可能性があるなら、承認の前に括弧の中の値を控えておきます。Actions が無効のリポジトリでは今の値を読み取れないため、括弧の中は値の代わりに `current values are unavailable` となり、値が消える可能性だけが示されます。

## 消えた `selected_actions` を戻すとき

控えておいた値を、`allowed_actions: selected` と一緒に書いて `apply` します。

```yaml
repositories:
  - name: example/project-a
    actions:
      permissions:
        allowed_actions: selected
        selected_actions:
          github_owned_allowed: true
          verified_allowed: true
          patterns_allowed:
            - "example/*"
```

## `all` や `local_only` から `selected` に変えるとき

GitHub は、`allowed_actions` が `selected` になるまで `selected_actions` の値を受け付けません。そのため reco は、先に `allowed_actions` を `selected` にし、そのあとで `selected_actions` を書いた値にする、という二回の更新を順に行います。

二回の更新のあいだは、`selected_actions` が設定ファイルと違う値になることがあります。

| 変更前 | 途中の状態の `selected_actions` |
|---|---|
| Actions が有効で、`all` か `local_only` | GitHub が決めた値（`github_owned_allowed: true`、`verified_allowed: false`、`patterns_allowed: []`） |
| Actions が無効で、無効にする前が `selected` | `github_owned_allowed` が `true` になるほかは、無効にする前の値のまま |

途中の状態では、変更前にも変更後にも許可しない Actions が、一時的に許可されることがあります。たとえば `local_only` から `github_owned_allowed: false` の `selected` に変えると、途中の状態のあいだだけ、GitHub が作った Actions が許可されます。

二回目の更新を始められなかったり失敗したりして `apply` が止まると、この途中の状態が残ります。そのときは `apply` を実行し直すと、`selected_actions` を書いた値にする変更が改めて計画されます。

## References

- [YAML 設定仕様](../spec/configuration.md)：`allowed_actions` と `selected_actions` の組み合わせ（4.1節）
- [CLI 仕様](../spec/cli.md)：副作用の表示（4章）
- [アーキテクチャ設計](../design/architecture.md)：`selected` に変えるときの二回の更新の順序と途中の状態（5.4節）
- [GitHub Actions 権限 API 仕様](../research/actions-permissions.md)：`selected_actions` が消える条件、途中の状態の値
