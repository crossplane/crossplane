---
title: "Crossplane Documentation"
weight: 2000
---
## Code of conduct
Crossplane follows the [CNCF Code of
Conduct](https://github.com/cncf/foundation/blob/main/code-of-conduct.md).

Taken directly from the code:
<!-- vale off -->
>As contributors and maintainers in the CNCF community, and in the interest of
>fostering an open and welcoming community, we pledge to respect all people who
>contribute through reporting issues, posting feature requests, updating
>documentation, submitting pull requests or patches, and other activities.
>  
>We are committed to making participation in the CNCF community a
>harassment-free experience for everyone, regardless of level of experience,
>gender, gender identity and expression, sexual orientation, disability,
>personal appearance, body size, race, ethnicity, age, religion, or nationality.
<!-- vale on -->

### Reporting violations
To report violations contact the Crossplane maintainers at `info@crossplane.io`
or the CNCF at `conduct@cncf.io`.

## Docs source 
Crossplane documentation lives in two repositories:

* Crossplane documentation source is in the [Crossplane
repository](https://github.com/crossplane/crossplane) `/docs` directory. 

* The Crossplane docs website is in the
  [crossplane.github.io](https://github.com/crossplane/crossplane.github.io)
repository.

Use `crossplane/crossplane` for documentation contributions.  
Use `crossplane/crossplane.github.io` for local development or updates involving
HTML, CSS or Hugo.

### Licensing
The Crossplane documentation is under the [Creative Commons
Attribution](https://creativecommons.org/licenses/by/4.0/) license. CC-BY allows
reuse, remixing and republishing of Crossplane documentation with attribution to
the Crossplane organization.

### Issues and feature requests
Open an [issue](https://github.com/crossplane/crossplane/issues)
to report a problem or request documentation content.

## Contributing

Documentation content contributions are always welcome.

The Crossplane documentation source is in the [Crossplane
repository](https://github.com/crossplane/crossplane) `/docs` directory. 


### Local development
Build the Crossplane documentation site locally for development and
testing. 

#### Clone the Crossplane repository
Clone the [crossplane
repository](https://github.com/crossplane/crossplane) with

```command
git clone https://github.com/crossplane/crossplane.git
```

#### Install Make

{{< tabs >}}

{{< tab "MacOS" >}}
Install `make` with [Homebrew](https://formulae.brew.sh/formula/make).

```command
brew install make
```
{{< /tab >}}

{{<tab "Linux" >}}
Most Linux distributions include `make` by default. For more information on
`make` for Linux, visit the [GNU make
website](https://www.gnu.org/software/make/).

{{< /tab >}}

{{<tab "Windows" >}}
Currently the Crossplane build system does not support Windows.
{{< /tab >}}

{{< /tabs >}}

#### Build the Crossplane documentation
From the `crossplane` folder run

```command
make docs.run
```

Hugo builds the website and launch a local web server on
[http://localhost:1313](http://localhost:1313).

Any changes made are instantly reflected on the local web server. You
don't need to restart Hugo.

### Contribute to a specific version
In the [crossplane/crossplane](https://github.com/crossplane/crossplane)
each active release has a `/docs` folder in a branch called
`release-<version>`. For example, v1.10 docs are in the branch
[release-1.10](https://github.com/crossplane/crossplane/tree/release-1.10).

To contribute to a specific release submit a pull-request to the
`release-<version>` or `master` branch.

The next Crossplane release uses `master` as the starting documentation.
## Style guidelines
The official Crossplane documentation style guide is still under construction.
Guiding principals for the documentation include:
<!-- vale off -->
* Avoid [passive voice](https://www.grammarly.com/blog/passive-voice/).
* Use [sentence-case headings](https://apastyle.apa.org/style-grammar-guidelines/capitalization/sentence-case).
* Wrap lines at 80 characters.
* Use [present tense](https://www.grammarly.com/blog/simple-present/).
* Spell out numbers less than 10, except for percentages, time and versions.
* Capitalize "Crossplane" and "Kubernetes." 
* Spell out the first use of an acronym unless it's common to new Crossplane
  users. When in doubt, spell it out first. 
* Don't use [cliches](https://www.topcreativewritingcourses.com/blog/common-cliches-in-writing-and-how-to-avoid-them).
* Use contractions for phrases like "do not", "cannot", "is not" and related terms.
* Don't use Latin terms (i.e., e.g., etc.).
* Don't use [gerund](https://owl.purdue.edu/owl/general_writing/mechanics/gerunds_participles_and_infinitives/index.html) headings (-ing words).
* Try and limit sentences to 25 words or fewer.
* [Be descriptive in link text](https://usability.yale.edu/web-accessibility/articles/links#link-text). Don't use "click here" or "read more".
<!-- vale on -->

Crossplane documentation is adopting
[Vale](https://github.com/errata-ai/vale) and relies on the [Upbound Vale
definitions](https://github.com/upbound/vale) for style guidelines.

Beyond Vale, Crossplane recommends [Grammarly](https://www.grammarly.com/) and [Hemingway
Editor](https://hemingwayapp.com/) to improve writing quality.

## Docs site styling features
The Crossplane documentation supports multiple styling features to improve
readability.

### Images
Crossplane supports standard [Markdown image
syntax](https://www.markdownguide.org/basic-syntax/#images-1) but using the
`img` shortcode is strongly recommended.

Images using the shortcode are automatically converted to `webp` image format,
compressed and use responsive image sizing. 

{{<hint "note">}}
The `img` shortcode doesn't support .SVG files.
{{< /hint >}}

The shortcode requires a `src` (relative to the file using the shortcode), an
`alt` text and an optional `size`.

The `size` can be one of:
* `xtiny` - Resizes the image to 150px.
* `tiny` - Resizes the image to 320px.
* `small` - Resizes the image to 600px.
* `medium` - Resizes the image to 1200px.
* `large` - Resizes the image to 1800px.

By default the image isn't resized.

An example of using the `img` shortcode:
```html
{{</* img src="../media/banner.png" alt="Crossplane Popsicle Truck" size="small" */>}}
```

Which generates this responsive image (change your browser size to see it change):
{{<img src="../media/banner.png" alt="Crossplane Popsicle Truck" size="small" >}}

### Links
Crossplane docs support standard [Markdown
links](https://www.markdownguide.org/basic-syntax/#links) but Crossplane prefers link shortcodes
for links between docs pages. Using shortcodes prevents incorrect link creation
and notifies which links to change after moving a page.

#### Between docs pages
For links between pages use a standard Markdown link in the form:

`[Link text](link)`

Crossplane recommends using the [Hugo ref
shortcode](https://gohugo.io/content-management/shortcodes/#ref-and-relref)
with the path of the file relative to `/content` for the link location.

For example, to link to the `master` release index page use
```markdown
[master branch documentation]({{</* ref "master/_index.md" */>}})
```

<!-- [master branch documentation]({{<ref "master/_index.md" >}}) -->

The `ref` value is of the markdown file, including `.md` extension.

If the `ref` value points to a page that doesn't exist, Hugo fails to start. 

#### Linking to external sites
Minimize linking to external sites. When linking to any page outside of
`crossplane.io` use standard [markdown link
syntax](https://www.markdownguide.org/basic-syntax/#links) without using the
`ref` shortcode.

For example, 
```markdown
[Go to Upbound](http://upbound.io)
```

### Tabs
Use tabs to present information about a single topic with multiple exclusive
options. For example, creating a resource via command-line or GUI. 

To create a tab set, first create a `tabs` shortcode and use multiple `tab`
shortcodes inside for each tab.

```html
{{</* tabs */>}}

{{</* tab "First tab title" */>}}
An example tab. Place anything inside a tab.
{{</* /tab */>}}

{{</* tab "Second tab title" */>}}
A second example tab. 
{{</* /tab */>}}

{{</* /tabs */>}}
```

This code block renders the following tabs
{{< tabs >}}

{{< tab "First tab title" >}}
An example tab. Place anything inside a tab.
{{< /tab >}}

{{< tab "Second tab title" >}}
A second example tab. 
{{< /tab >}}

{{< /tabs >}}

Both `tab` and `tabs` require opening and closing tags. Unclosed tags causes
Hugo to fail.

### Hints and alert boxes
Hint and alert boxes provide call-outs to important information to the reader. Crossplane docs support four different hint box styles.

{{< hint "note" >}}
Notes are useful for calling out optional information.
{{< /hint >}}

{{< hint "tip" >}}
Use tips to provide context or a best practice.
{{< /hint >}}

{{< hint "important" >}}
Important hints are for drawing extra attention to something. 
{{< /hint >}}

{{< hint "warning" >}}
Use warning boxes to alert users of things that may cause outages, lose data or
are irreversible changes.
{{< /hint >}}


Create a hint by using a shortcode in your markdown file:
```html
{{</* hint "note" */>}}
Your box content. This hint box is a note.
{{</* /hint */>}}
```

Use `note`, `tip`, `important`, or `warning` as the `hint` value.

The `hint` shortcode requires opening and closing tags. Unclosed tags causes
Hugo to fail.


### Hide long outputs
Some outputs may be verbose or only relevant for
a small audience. Use the `expand` shortcode to hide blocks of text by default.

{{<expand "A large XRD" >}}
```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: CompositeResourceDefinition
metadata:
  name: xpostgresqlinstances.database.example.org
spec:
  group: database.example.org
  names:
    kind: XPostgreSQLInstance
    plural: xpostgresqlinstances
  claimNames:
    kind: PostgreSQLInstance
    plural: postgresqlinstances
  versions:
  - name: v1alpha1
    served: true
    referenceable: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          spec:
            type: object
            properties:
              parameters:
                type: object
                properties:
                  storageGB:
                    type: integer
                required:
                - storageGB
            required:
            - parameters
```
{{< /expand >}}

The `expand` shortcode can have a title, the default is "Expand."
````yaml
{{</* expand "A large XRD" */>}}
```yaml
apiVersion: apiextensions.crossplane.io/v1
kind: CompositeResourceDefinition
metadata:
  name: xpostgresqlinstances.database.example.org
```
{{</* /expand */>}}
````

The `expand` shortcode requires opening and closing tags. Unclosed tags causes
Hugo to fail.

## Adding new content

To create new content create a new markdown file in the appropriate location. 

To create a new section, create a new directory and an `_index.md` file in the
root. 

### Front matter
Each page contains metadata called [front matter](https://gohugo.io/content-management/front-matter/). Each page requires front matter to render.

```yaml
---
title: "A New Page"
weight: 610
---
```

`title` defines the name of the page.
`weight` determines the ordering of the page in the table of contents. Lower weight pages come before higher weights in the table of contents. The value of `weight` is otherwise arbitrary.


### Hiding pages
To hide a page from the left-hand navigation use `tocHidden: true` in the front
matter of the page. The docs website skips pages with `tocHidden:true` when building the menu.

## Docs website
The Crossplane document website is in a unique [website GitHub
repository](https://github.com/crossplane/crossplane.github.io).

Crossplane uses [Hugo](https://gohugo.io/), a static site generator. Hugo
influences the directory structure of the website repository.

The `/content` directory is the root directory for all documentation content.

The `/themes/geekboot` directory is the root directory for all website related
files, like HTML templates, shortcodes and global media files. 

The `/utils/` directory is for JavaScript source code used in the website. 

The `/themes/geekboot/assets` folder contains all (S)CSS and compiled JavaScript
for the website.

### CSS
Crossplane documentation uses [Bootstrap
5.2](https://getbootstrap.com/docs/5.2/getting-started/introduction/).
Unmodified Bootstrap SCSS files are in
`/themes/geekboot/assets/scss/bootstrap/`. Any docs-specific overrides are in
per-element SCSS files located one directory higher in
`/themes/geekboot/assets/scss/`.

{{<hint "important" >}}
Don't edit the original Bootstrap stylesheets. It makes the ability to
upgrade to future Bootstrap versions difficult or impossible.
{{< /hint >}}

#### Color themes 
Crossplane docs support a light and dark color theme that's applied via CSS
variables.

Universal and default variables are defined in
`/themes/geekboot/assets/scss/_variables.scss`.

Provide theme specific color overrides in
`/themes/geekboot/assets/scss/light-mode.scss` or
`/themes/geekboot/assets/scss/light-mode.scss`.

{{<hint "note" >}}
When creating new styles rely on variables for any color function, even if both
themes share the color.
{{< /hint >}}

#### SCSS compilation
Hugo compiles the SCSS to CSS. Local development doesn't require SCSS installed.

For local development (when using `hugo server`) Hugo compiles SCSS without
any optimizations.

For production (publishing on Netlify or using `hugo server
--environment production`) Hugo compiles SCSS and optimizes the CSS with
[PostCSS](https://postcss.org/). The PostCSS configuration is in
`/postcss.config.js`. The optimizations includes:
* [postcss-lightningcss](https://github.com/onigoetz/postcss-lightningcss) - for
  building, minimizing and generating a source map.
* [PurgeCSS](https://purgecss.com/plugins/postcss.html) - removes unused styles
  to reduce the CSS file size. 
* [postcss-sort-media-queries](https://github.com/yunusga/postcss-sort-media-queries)- 
to organize and reduce CSS media queries to remove duplicate and unused
    CSS.

Optimizing CSS locally with PostCSS requires installing extra packages.
* [Sass](https://sass-lang.com/install)
* [NPM](https://www.npmjs.com/)
* NPM packages defined in `/package.json` with `npm install`.


### JavaScript
A goal of the documentation website is to use as little JavaScript as possible. Unless
the script provides a significant improvement in performance, capability or user
experience. 

To make local development there are no run-time dependencies for
JavaScript. 

Runtime JavaScript is in `/themes/geekboot/assets/js/`. [Webpack](https://webpack.js.org/)
has bundled, minified and compressed the JavaScript.

The source JavaScript is in `/utils/webpack/src/js` and
requires [Webpack](https://webpack.js.org/) to bundle and optimize the code.

* `colorMode.js` provides the ability to change the light/dark mode color theme.
* `tabDeepAnchor.js` rewrites anchor links inside tabs to open a tab and present
  the anchor. 
* `globalScripts.js` is the point of entry for Webpack to determine all
  dependencies. This bundles [instant.page](https://instant.page/) and
  [Bootstrap's
  JavaScript](https://getbootstrap.com/docs/5.2/getting-started/javascript/).
  
#### Bootstrap JavaScript
The entire [Bootstap JavaScript
source](https://github.com/twbs/bootstrap/tree/main/js/src) is in
`/utils/webpack/src/js/bootstrap`. 

Adding a new Bootstrap feature requires importing it in `globalScripts.js`. 

By importing only the necessary Bootstrap JavaScript features, reduces the
bundle size.
## Annotated website tree
Expand the tab below to see an annotated `tree` output of the website repo.

{{<expand >}}
```shell
├── content                     # Root for all page content
│   ├── master
│   ├── v1.10
│   ├── v1.8
│   └── v1.9
├── themes                      # Entry point for theme-specific designs
│   └── geekboot
│       ├── assets              # JS and stylesheets
│       │   ├── js              # Bundled and optmized Javascript
│       │   └── scss            # Bootstrap SCSS overrides
│       │       └── bootstrap   # Bootstrap original SCSS files
│       ├── data
│       ├── layouts             # HTML layouts and shortcodes
│       │   ├── _default        # HTML layouts for page types
│       │   │   └── _markup     # Hugo render hooks
│       │   ├── partials        # HTML Template elements
│       │   │   ├── icons
│       │   │   └── utils
│       │   └── shortcodes      # Shortcode features
│       └── static              # Static files across the theme.
│           ├── fonts           # Font files
│           └── img             # Global images
└── utils                       # Files unrelated to Hugo
    └── webpack                 # Files managed or related to webpack
        └── src
            └── js
                └── bootstrap/  # Original Bootstrap JavaScript
                └── colorMode.js  # Color theme switcher
                └── tabDeepAnchor.js # Enable anchors inside tabs
```
{{</expand>}}