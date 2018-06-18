package main

var rootPage = `
<html>
<meta charset="utf-8">

<head>
<title>Kubernetes app demo</title>
<link rel="stylesheet" href="https://maxcdn.bootstrapcdn.com/bootstrap/4.0.0/css/bootstrap.min.css"
integrity="sha384-Gn5384xqQ1aoWXA+058RXPxPg6fy4IWvTNh0E263XmFcJlSAwiGgFAW/dAiS6JXm" crossorigin="anonymous">
<style>
td {
	font-size: 70%;
}
a {
	font-weight: bold;
}
body {
	padding: 10px
}
</style>
</head>

<body>
<div class="container">
<div class="row">

<div class="col-sm-6">

{{ if .Hits }}
<div class="alert alert-info">This worker returned page <strong>{{ .Hits }}</strong> times.</div>
{{ end }}


<div class="alert alert-info">Metrics exported at <a href="/metrics">/metrics</a></div>


{{ if .RedisHost }}
<div class="alert alert-info">Redis host is <code>{{ .RedisHost }}</code></div>
{{ else }}
<div class="alert alert-info">Redis server not used.</div>
{{ end }}

</div>

<div class="col-sm-6">
<table class="table table-hover">
<thead>
<tr><th>Variable name</th><th>Value</th></tr>
</thead>
<tbody>
{{ range $v := .Vars }}
<tr class="{{ if $v.Dangerous }}table-danger{{ end }}"><td>{{ $v.Name }}</td><td>{{ $v.Value }}</td></tr>
{{ end }}
</tbody>
</table>
</div>

</div> <!-- row -->
</div> <!-- container -->

</body>
</html>
`
