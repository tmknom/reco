# GitHub の OpenAPI 記述から、$operations に挙げた操作と、それらが推移的に参照する components だけを残す。
# $operations は {"<path>": ["<method>", ...]} の形の JSON。
# path レベルの parameters は、残した操作が使うため常に残す。

def pointer_path: ltrimstr("#/") | split("/") | map(gsub("~1"; "/") | gsub("~0"; "~"));
def refs_in: [.. | objects | .["$ref"]? // empty | strings] | unique;

. as $root
| ($operations | to_entries | map(
    .key as $path
    | ($root.paths[$path] // error("path not found: \($path)")) as $item
    | {
        key: $path,
        value: (
          ($item | with_entries(select(.key == "parameters")))
          + (.value | map({key: ., value: ($item[.] // error("operation not found: \(.) \($path)"))}) | from_entries)
        )
      }
  ) | from_entries) as $paths
| def closure($seen):
    ($seen + ([$seen[] as $r | $root | getpath($r | pointer_path) | refs_in] | add // []) | unique) as $all
    | if $all == $seen then $all else closure($all) end;
  closure($paths | refs_in) as $refs
| reduce ($refs[] | pointer_path) as $p (
    {openapi: $root.openapi, info: $root.info, servers: $root.servers, externalDocs: $root.externalDocs, paths: $paths};
    setpath($p; $root | getpath($p))
  )
