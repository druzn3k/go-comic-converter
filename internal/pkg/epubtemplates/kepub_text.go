package epubtemplates

// KepubText is the XHTML page template for KEPUB.
// Key difference from EPUB: images are wrapped in <div class="kobolink">
// to enable Kobo's panel zoom feature.
var KepubText string = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
  <head>
    <meta charset="utf-8" />
    <title>{{ .Title | xmlEscape }}</title>
    <link href="style.css" type="text/css" rel="stylesheet"/>
    <meta name="viewport" content="{{ .ViewPort }}"/>
  </head>
  <body>
    <div class="kobolink"><img src="{{ .ImagePath }}" alt="{{ .Title | xmlEscape }}" style="{{ .ImageStyle }}"/></div>
  </body>
</html>`
