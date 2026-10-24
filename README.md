# specrig

The Claude Code plugin of [specrig](https://github.com/specrig/protocol): it
enforces Rule #11's modes for every service a marker names — `specrig.yaml` or
`waffle.yaml` — whatever its stack.

| | what |
|---|---|
| [guard](guard) | the verdict for one service from its rules, rules/1: whether the mode on allows a path, and every way to one it refuses. [protocol](protocol) runs the protocol's fixtures against it |
| [service](service) | the service of a path and the tool its marker names: its rules read once a session, or guard/1's commands asked of a tool older than them |
| [claude](claude) | the Claude Code side: the hooks, the `/specrig:` skills, and the mode a session holds |

## Install

```
/plugin marketplace add specrig/specrig
/plugin install specrig@specrig
```

For everyone on a project, in the repository root's `.claude/settings.json`:

```json
{
  "extraKnownMarketplaces": {
    "specrig": {"source": {"source": "github", "repo": "specrig/specrig"}}
  },
  "enabledPlugins": {"specrig@specrig": true}
}
```

## Modes

The human switches with `/specrig:spec`, `/specrig:redgreen`, `/specrig:platform`
and `/specrig:none`, or with `/specrig:auto` for the whole cycle without a stop,
naming the services by path — `./services/shop`, or `./...` for every service below
a directory — and grants an area with `/specrig:allow <area>`. One mode holds for
every service the task names, and a service it does not name is closed whole.

The rules come from the tool version the service depends on: the hook asks the tool
the marker names for them once a session, and again when the marker changes, and
checks every edit against them itself.

| marker | asked |
|---|---|
| `specrig.yaml` with `guard: [dotnet, specrig]` | `dotnet specrig rules` |
| `specrig.yaml` with `guard: [npx, --no, specrig]` | `npx --no specrig rules` |
| `waffle.yaml` | `./waffle agents:rules`, or `go tool waffle agents:rules` |

A tool older than `rules` is asked `guard <mode> <path>` on every edit instead, and
`mode` and `grant` when the human switches. A service whose tool has no modes yet,
or does not run, stays editable, and the plugin says so once. The plugin supersedes
waffle's own, which enforces the same table for waffle services alone: enable one of
the two, not both.

**No Go needed.** The hook is a Go program. Where Go is installed the plugin builds
it on first use; where it is not, it fetches the binary for the platform from the
release of the plugin's version and checks it against the release's checksums. A
hook that cannot be had lets every edit through and says so at the start of the
session.

## Development

```bash
make test
```

runs the packages, and the protocol's fixtures against the copy of the protocol in
`protocol`.

```bash
make protocol
```

refreshes that copy from a checkout of the protocol at `PROTOCOL`, `../protocol` by
default.

```bash
make release
```

builds the hook for darwin and linux, arm64 and amd64, into `dist/` with its
checksums; a tag `v<version>` matching `.claude-plugin/plugin.json` publishes them
through `.github/workflows/release.yml`.

## License

[MIT](LICENSE)
