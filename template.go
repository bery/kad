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
	padding: 10px;
	background-color: {{ .Color }};
}
div.doc {
	font-size: 90%;
	background-color: #fafafa;
	padding: 10px;
	border-radius: 2px;
	font-weight: 400;
}

li.pod a, li.deploy a, li.rs a, li.svc a {
	display: block;
	padding: 2px;
	margin: 2px;
	color: white;
	text-align: center;
}

li a, li a:hover {
	color: white;
	cursor: crosshair;
	text-decoration: none;
}

li.pod {
	background-color: #050505;
}
li.deploy {
	background-color: #ffc674;
}
li.rs {
	background-color: #f0a9d5
}
li.svc {
	background-color: #dcea64;
}
</style>
{{ if .PageRefresh }}
<meta http-equiv="refresh" content="2">
{{ end }}
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
<div class="alert alert-info">Redis server is <code>{{ .RedisHost }}</code>, path <code>{{ .RedisPath }}</code></div>
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

{{ if not .Ready }}
<div class="alert alert-danger">This replica isn't ready.</div>
{{ end }}


{{ if .KubernetesError }}
<div class="alert alert-warning">Failed accesing Kubernetes: <code>{{ .KubernetesError }}</code></div>
{{ end }}

<div class="doc">
<b>Endpoints (port 5000):</b>
<ul>
	<li><a>/heavy</a> - run many parallel goroutines printing /dev/null</li>
	<li><a>/slow</a> - wait 3 second before server reply</li>
	<li><a>/check/live</a> - liveness probe, always OK</li>
	<li><a>/check/ready</a> - readiness probo, ready if file <code>/tmp/notready</code> doesn't exist</li>
	<li><a>/metrics</a> - <a href="https://prometheus.io/">Prometheus</a> metrics</li>
	<li><a>/hostname</a> - prints hostname
</ul>

<b>Admin endpoints (port 5001):</b>
<ul>
	<li><a>/action/terminate</a> - Disable readiness probe, wait 15s and exit</li>
	<li><a>/check/live</a> - liveness probe, always OK</li>
	<li><a>/check/ready</a> - readiness probo, ready if file <code>/tmp/notready</code> doesn't exist</li>
</ul>

<b>Command options:</b>
<ul>
	<li><a>--color</a> - Set background color</li>
</ul>


<p>
Server is expecting configuration file <code>{{ .ConfigFilePath }}</code>. It will run without configuration but error mesage will be printed.
</p>

<p>
Contribution is apprecited at <a href="https://gitlab.com/6shore.net/kad">gitlab.com/6shore.net/kad</a>
</p>

</div>


</div>
<div class="col-sm-6">

{{ if not .KubernetesError }}
<div class="doc">

<p>
Kubernetes at <a href="{{ .KubernetesHost }}">{{ .KubernetesHost }}</a>
</p>

Pods
<ul>
{{ range $i := .Resources.Pods }}
<li class="pod"><a href="/kubernetes/delete/pod/{{ $i.ObjectMeta.Name }}">{{ $i.ObjectMeta.Name }}</a></li>
{{ end }}
</ul> 

Deployments
<ul>
{{ range $i := .Resources.Deployments }}
<li class="deploy"><a href="/kubernetes/delete/deploy/{{ $i.ObjectMeta.Name }}">{{ $i.ObjectMeta.Name }}</a></li>
{{ end }}
</ul> 

ReplicaSets
<ul>
{{ range $i := .Resources.ReplicaSets }}
<li class="rs"><a href="/kubernetes/delete/rs/{{ $i.ObjectMeta.Name }}">{{ $i.ObjectMeta.Name }}</a></li>
{{ end }}
</ul> 

Services
<ul>
{{ range $i := .Resources.Services }}
<li class="svc"><a href="/kubernetes/delete/svc/{{ $i.ObjectMeta.Name }}">{{ $i.ObjectMeta.Name }}</a></li>
{{ end }}
</ul> 

</div>
{{ end }}

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
