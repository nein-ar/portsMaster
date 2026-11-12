# portsMaster

Reads an `spc` port tree and writes a static site.

`portsMaster` turns ports, git history, CI data and package files into a content
tree of `.djot` files and JSON data files. It then hands that tree to the `ssg`
package in [`dejot`](https://github.com/nein-ar/dejot).

The split keeps each side small. The site generator only sees pages and data.
The port collector only sees ports and git.

## Build

```sh
make
./portsMaster --config config.toml
```

Flags:

```sh
./portsMaster --config config.toml --theme xplshn
./portsMaster --config config.toml --serve --watch
```

## Themes

The config sets the default theme:

```toml
theme = "shrub"
```

`shrub` is Shrub's design and the default. `xplshn` is xplshn's design. The
command line flag `--theme xplshn` overrides the config and stores the choice in
`.cache/theme`.

Both themes build the same pages. They differ in CSS and in a few blocks that
one design shows and the other hides. Every page loads
`assets/theme-switch.js`. When JavaScript is available, the footer shows:

```text
Press [C] to change theme. Currently using Shrub's.
```

Pressing `C` switches to the other design, updates the text, and stores the
choice in `localStorage`. A visitor without JavaScript sees the theme from the
config or the last `--theme` flag. The switcher stays hidden.

The shared markup lives in `themes/base/`. `themes/shrub/` and
`themes/xplshn/` only declare the default design. The skins are in
`themes/base/static/assets/shrub/` and `themes/base/static/assets/xplshn/`.

## Sources and outputs

| Path | Meaning |
| --- | --- |
| `content/` | generated Djot sources |
| `data/` | generated JSON data |
| `public/` | generated site |
| `.cache/` | parse, git, CI, log and build caches |

Git ignores all four paths. Deleting any of them is safe. The next run rebuilds
them from the port tree.

## Caching

Each stage has a content key:

- the cache hashes port directories, and unchanged ports come from a parsed copy
- the cache keys git history by commit id and port set
- remote CI status uses `ETag` and `Last-Modified`, and a `304` reuses the body
- the cache keys build logs by build timestamp
- the generator writes Djot and JSON files only when their bytes change
- the site generator compares a fingerprint of its inputs before it parses Djot
- the build removes stale cache entries, sources and output files

A second run on an unchanged tree prints `site: up to date` and does no site
work.

## Port formats

The parser reads the existing `spc` files under
`ports/<category>/<name>/`:

- `info`: `name`, `version`, `release`, `description`, `license`, `upstream`,
  `maintainer`, `provides`
- `deps`: `.` build, `>` run, `/` link, a bare name link
- `ndmake.sh`: counted for recipe complexity
- the generator embeds every regular file in a file viewer page
- a `BROKEN` marker marks the port broken

CI, packages and git history are optional. A missing source does not fail the
build. The theme renders what it has.

## Templates

Templates are Go `html/template` files. The context contains:

- `.Site`: title, description, params, data, pages
- `.Page`: the current page, URL, path, params and dates
- `.Content`: the page's rendered Djot
- `.Body`: a layout's rendered body, used by the base template
- `.Data`: the data files, also available as `.Site.Data`

See the `dejot` README for the template contract.
