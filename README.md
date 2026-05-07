# bbgit

A Git web interface written in Go.

## Features

- **Repository Management**: Supports multiple repositories grouped by category in `config.yaml`.
- **Browsing**: Navigate commit history, directory trees, and file contents.
- **Syntax Highlighting**: Automatic language detection and highlighting via [Chroma](https://github.com/alecthomas/chroma).
- **Markdown Rendering**: Renders README files with GFM support and GitHub-style alerts via [Goldmark](https://github.com/yuin/goldmark).
- **Language Statistics**: Visual breakdown of programming languages used in each repository.
- **Marker Scanning**: Scans for `TODO:` and `FIXME:` comments across repository source files.
- **Diffs & Patches**: Side-by-side diff view for commits and raw patch export.
- **Archives**: Download repository snapshots as `.tar.gz`.
- **RSS Feeds**: Commit history and tag updates delivered via standard RSS feeds.
- **Caching**: Persistent caching of commit statistics and repository metadata to improve performance.

## Getting Started

1. **Configure**: Define your repositories in `config.yaml`:
   ```yaml
   repositories:
     - name: "my-project"
       path: "/path/to/repo"
       description: "Project description"
       group: "Category"
   ```
2. **Run**:
   ```bash
   go run .
   ```
3. **Access**: Open http://localhost:8080

## Configuration

The `config.yaml` file defines the repositories served by `bbgit`:

| Field | Description |
|-------|-------------|
| `name` | Unique identifier used in the URL (`/r/{name}`). |
| `path` | Absolute path to the local Git repository. |
| `description` | Short text displayed on the repository index. |
| `group` | Category name used to group repositories on the home page. |

## Deployment (systemd)

1. Copy the service file:
   ```bash
   sudo cp bbgit.service /etc/systemd/system/
   ```
2. Edit `/etc/systemd/system/bbgit.service` to set the correct `User`, `Group`, `WorkingDirectory`, and `ExecStart` paths.
3. Reload and start:
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable --now bbgit
   ```

## Technical Details

- **Language**: Go 1.22+
- **No external JS**: Server-side rendered using Go templates.
- **Metadata Cache**: Stored in `bbgit.cache`. Persistent data is saved on `SIGINT` or `SIGTERM`.
- **Build**: Compile a static binary with `go build -o bbgit .`.

## Routing Table

| Endpoint | Description |
|----------|-------------|
| `/` | Home page with grouped repository list. |
| `/r/{name}` | Commit history (log) for the repository. |
| `/r/{name}/tree/{path}` | Directory tree browser. |
| `/r/{name}/blob/{path}` | File viewer with syntax highlighting. |
| `/r/{name}/raw/{path}` | Raw file download. |
| `/r/{name}/markers` | Results of the `TODO:` and `FIXME:` scanner. |
| `/r/{name}/commits.rss` | RSS feed for commit history (supports `?ref=`). |
| `/r/{name}/tags.rss` | RSS feed for repository tags. |
| `/r/{name}/archive/{path}` | Snapshot download in `.tar.gz` format. |
| `/r/{name}/c/{hash}` | Commit detail view with side-by-side diff. |
| `/r/{name}/c/{hash}/patch` | Raw patch file for the commit. |

## Libraries

- [go-git](https://github.com/go-git/go-git): Git implementation in Go.
- [Chroma](https://github.com/alecthomas/chroma): General purpose syntax highlighter.
- [Goldmark](https://github.com/yuin/goldmark): Markdown parser and renderer.
- [go-enry](https://github.com/go-enry/go-enry): Programming language detection.
- [yaml.v3](https://github.com/go-yaml/yaml): YAML support for configuration.

## License

BSD 2-Clause License. See `LICENSE` for details.
