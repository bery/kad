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
div.doc {
	font-size: 90%;
	background-color: #fafafa;
	padding: 10px;
	border-radius: 2px;
	font-weight: 400;
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
<div class="alert alert-info">Redis server is <code>{{ .RedisHost }}</code></div>
{{ else }}
<div class="alert alert-warning">Redis server not used, set <code>REDIS_SERVER</code> to use it.</div>
{{ end }}

{{ if ne .RedisError "" }}
<div class="alert alert-danger">Redis connection failed: <code>{{ .RedisError }}</code></div>
{{ end }}

{{ if .Cmd }}
<div class="alert alert-info">Started with command <code>{{ .Cmd }}</code></div>
{{ end }}

{{ if .ConfFile }}
<div class="alert alert-info">Config file content:<br><code><pre>{{ .ConfFile }}<pre></code></div>
{{ else }}
<div class="alert alert-warning">Config file <code>/etc/kad/config.yml</code> is empty.</code></div>
{{ end }}

<div class="doc">
<b>Endpoints (port 5000):</b>
<ul>
	<li><a href="{{ .Request.URL.Path }}heavy">/heavy</a> - run many parallel goroutines printing /dev/null</li>
	<li><a href="{{ .Request.URL.Path }}slow">/slow</a> - wait 3 second before server reply</li>
	<li><a href="{{ .Request.URL.Path }}check/live">/check/live</a> - liveness probe, always OK</li>
	<li><a href="{{ .Request.URL.Path }}check/ready">/check/ready</a> - readiness probo, ready if file <code>/tmp/notready</code> doesn't exist</li>
	<li><a href="{{ .Request.URL.Path }}metrics">/metrics</a> - <a href="https://prometheus.io/">Prometheus</a> metrics</li>
</ul>

<b>Admin endpoints (port 5001):</b>
<ul>
	<li><a href="{{ .Request.URL.Path }}action/terminate">/action/terminate</a> - Disable readiness probe, wait 15s and exit</li>
	<li><a href="{{ .Request.URL.Path }}check/live">/check/live</a> - liveness probe, always OK</li>
	<li><a href="{{ .Request.URL.Path }}check/ready">/check/ready</a> - readiness probo, ready if file <code>/tmp/notready</code> doesn't exist</li>
</ul>

<p>
Server is expecting configuration file <code>{{ .ConfigFilePath }}</code>. It will run without configuration but error mesage will be printed.
</p>








</div>



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
