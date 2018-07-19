{{define "header"}}

<!DOCTYPE html>
<html lang="en">

<head>
  <meta http-equiv="Content-Type" content="text/html; charset=UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1, maximum-scale=1.0" />
  <title>Starter Template - Materialize</title>

  <!-- CSS  -->
  <link href="https://fonts.googleapis.com/icon?family=Material+Icons" rel="stylesheet">
  <link href="/css/materialize.min.css" type="text/css" rel="stylesheet" media="screen,projection" />
  <link href="/css/style.css" type="text/css" rel="stylesheet" media="screen,projection" />
</head>

<body class="">
  <nav class="green darken-4" role="navigation">
    <div class="nav-wrapper container">
      <div class="hide-on-med-and-down">
        <a href="/" class="brand-logo">Aidos Explorer</a>
      </div>
      <ul class="right">
        <li>
          <div class="center row">
<form action="/search" method="GET">
            <div class="input-field black-text">
              <i class="white-text material-icons prefix">search</i>
              <input type="text" name="id" class="white-text">
            </div>
</form>
          </div>
        </li>
      </ul>
  </nav>

{{end}}

