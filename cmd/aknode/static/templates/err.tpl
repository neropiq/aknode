{{define "err"}}
	{{template "header" .}}

  <div class="section no-pad-bot" id="index-banner">
    <div class="container">
      <h1 class="header center black-text">
        <img class="responsive-img" src="/image/logo-min.png"> </h1>
    </div>
  </div>


  <div class="container">
    <div class="section">
      <h4>Sorry, {{.}}</h4>
    </div>
    <div class="row">
<form action="/search" method="GET">
      <div class="input-field black-text">
        <i class="black-text material-icons prefix">search</i>
        <input name="id" type="text" class="black-text">
      </div>
</form>
    </div>
  </div>

	{{template "footer" .}}
{{end}}
