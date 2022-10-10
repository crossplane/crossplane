---
title: "Crossplane Documentation"
weight: 2000
---

- [Code of Conduct](#code-of-conduct)
  - [Reporting](#reporting)
- [Licensing](#licensing)
  - [Directory structure](#directory-structure)
  - [Run Hugo](#run-hugo)
  - [Documentation issues and feature requests](#documentation-issues-and-feature-requests)
- [Contributing](#contributing)
  - [Front matter](#front-matter)
    - [New Crossplane versions](#new-crossplane-versions)
    - [Excluding pages from the Table of Contents](#excluding-pages-from-the-table-of-contents)
  - [Links](#links)
    - [Linking between docs pages](#linking-between-docs-pages)
    - [Linking to external sites](#linking-to-external-sites)
  - [Tabs](#tabs)

## Code of Conduct
We follow the [CNCF Code of Conduct](https://github.com/cncf/foundation/blob/main/code-of-conduct.md).

Taken directly from the CNCF CoC:
>As contributors and maintainers in the CNCF community, and in the interest of fostering an open and welcoming community, we pledge to respect all people who contribute through reporting issues, posting feature requests, updating documentation, submitting pull requests or patches, and other activities.
>  
>We are committed to making participation in the CNCF community a harassment-free experience for everyone, regardless of level of experience, gender, gender identity and expression, sexual orientation, disability, personal appearance, body size, race, ethnicity, age, religion, or nationality.

### Reporting
To report violations contact the Crossplane maintainers at `info@crossplane.io` or the CNCF at `conduct@cncf.io`.

## Licensing
The Crossplane documentation is under the [Creative Commons Attribution](https://creativecommons.org/licenses/by/4.0/) license. CC-BY allows reuse, remixing and republishing of Crossplane documentation with attribution to the Crossplane organization.


### Directory structure
The relevant directories of the docs repository are:
| Directory | Purpose |
| ----- | ---- |
| `content` | Markdown files for all individual pages. |
| `static/images` | Location for all image files.  |
| `themes` | HTML templates and Hugo tooling. |

All markdown content exists within the `/content` directory.   
All image files exist within `static/images`.

### Run Hugo
* Follow the [Hugo documentation](https://gohugo.io/getting-started/installing/) to install Hugo.
* Run `hugo server`
* Go to [http://localhost:1313](http://localhost:1313)

Crossplane documentation uses [Hugo v0.101.0](https://github.com/gohugoio/hugo/releases/tag/v0.101.0), but newer versions are compatible.

### Documentation issues and feature requests
Use [GitHub Issues](https://github.com/crossplane/crossplane.github.io/issues) to report a problem or request documentation content.

## Contributing

The `/content` directory contains all documentation pages.  
Each version of Crossplane software maintains a unique directory within `content`.

### Front matter
Each page contains metadata called [front matter](https://gohugo.io/content-management/front-matter/). Each page requires front matter to render.

```yaml
---
title: "Troubleshooting Crossplane"
weight: 610
---
```

`title` defines the name of the page.  
`weight` determines the ordering of the page in the table of contents. Lower weight pages come before higher weights in the table of contents. The value of `weight` is otherwise arbitrary.

The `weight` of a directory's `_index.md` page moves the entire section of content in the table of contents.

#### New Crossplane versions
To create documentation for a new version of Crossplane copy the current `master` folder and rename it to the desired version. 
The naming convention follows `v<major>.<minor>`. Maintenance release and individual builds do not have unique documentation. 

After creating a new directory edit the `/content/<version>/_index.md` file to set the version for all sub-pages define `verison:` as a child element under `cascade:`. 
For example, set the version to `v8.6`:
```
---
title: "Overview"
weight: 1
toc_include: false
cascade:
    version: 8.6
---
```

You **must** set the version number as a valid float to generate the version drop down menus within the docs.

The version set in the front matter is independent from the directory name.

#### Excluding pages from the Table of Contents
Pages can be hidden from the left-side table of contents with front matter settings. 

To exclude a page from the table of contents set  

`toc_include: false`  

in the page front matter. 

For example, 

```
---
title: "Overview"
weight: 1
toc_include: false
---
```

The `/content/<version>/_index.md` page must have this set.

### Links
#### Linking between docs pages
Link between pages or sections within the documentation using standard [markdown link syntax](https://www.markdownguide.org/basic-syntax/#links). Use the [Hugo ref shortcode](https://gohugo.io/content-management/shortcodes/#ref-and-relref) with the path of the file relative to `/content` for the link location.

For example, to link to the "Official Providers" page create a markdown link like this:

```markdown
[a link to the Official Providers page]({{</* ref "providers/_index.md" */>}})
```

The `ref` value is of the markdown file, including `.md` extension. Don't link to a web address.

If the `ref` value points to a page that doesn't exist, Hugo fails to start. 

For example, providing `index.md` instead of `_index.md` would cause an error like the this:
```shell
Start building sites …
hugo v0.101.0-466fa43c16709b4483689930a4f9ac8add5c9f66 darwin/arm64 BuildDate=2022-06-16T07:09:16Z VendorInfo=gohugoio
ERROR 2022/09/13 13:53:46 [en] REF_NOT_FOUND: Ref "contributing/index.md": "/home/user/crossplane.github.io/content/contributing.md:64:41": page not found
Error: Error building site: logged 1 error
```

Using `ref` ensures links render in the staging environment and prevents broken links.

#### Linking to external sites
Minimize linking to external sites. When linking to any page outside of `crossplane.io` use standard [markdown link syntax](https://www.markdownguide.org/basic-syntax/#links) without using the `ref` shortcode.

For example, 
```markdown
[Go to Upbound](http://upbound.io)
```

### Tabs
Use tabs to present information about a single topic with multiple exclusive options. For example, creating a resource via command-line or GUI. 

To create a tab set, first create a `tabs` shortcode and use multiple `tab` shortcodes inside for each tab.

Each `tabs` shortcode requires a name that's unique to the page it's on. Each `tab` name is the title of the tab on the webpage. 

For example
```
{{</* tabs "my-unique-name" */>}}

{{</* tab "Command-line" */>}}
An example tab. Place anything inside a tab.
{{</* /tab */>}}

{{</* tab "GUI" */>}}
A second example tab. 
{{</* /tab */>}}

{{</* /tabs */>}}
```

This code block renders the following tabs
{{< tabs "my-unique-name" >}}

{{< tab "Command-line" >}}
An example tab. Place anything inside a tab.
{{< /tab >}}

{{< tab "GUI" >}}
A second example tab. 
{{< /tab >}}

{{< /tabs >}}


Both `tab` and `tabs` require opening and closing tags. Unclosed tags causes Hugo to fail.

