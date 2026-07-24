import MarkdownIt from 'markdown-it'

// One shared markdown-it instance for the whole app. The how-to / form-cues /
// common-faults fields are authored as markdown; this renders them to HTML.
// linkify turns bare URLs (a movement's source) into links; typographer smartens
// quotes and dashes. Movement content is first-party (seeded or entered by the
// single user), so raw HTML stays disabled (the markdown-it default) — no
// sanitizer needed for untrusted input because there is none.
const md = new MarkdownIt({
  html: false,
  linkify: true,
  typographer: true,
  breaks: true,
})

export function renderMarkdown(source: string): string {
  return md.render(source ?? '')
}
