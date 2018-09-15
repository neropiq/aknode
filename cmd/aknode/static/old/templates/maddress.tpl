{{define "maddress"}}
	{{template "header" .}}

  <div class="container">
    <div class="section">
      <div class="center">
        <h4>Multisig Address</h4>
        <img src="/qrcode?id={{.Address}}">
        <a class="truncate" href="/address?id={{.Address}}">{{.Address}}</a>
        <a class="balance">Balance: {{toADK .Balance}} ADK</a>
            <div class="tx small-address truncate">
              {{.Struct.M}} out of {{len .Struct.Addresses}}
              <br>
			{{range .Struct.Addresses}}
              <a href="/address?id={{.String}}">{{.String}}</a><br>
			{{end}}
            </div>

      </div>
      <br>
      <ul class="collapsible">
          <li>
            <div class="collapsible-header">UTXOs</div>
            <div class="collapsible-body">
			{{range .UTXOs}}
                <div class="row">
                    <div class="col s12 m8">
                      <a class="truncate" href="/tx?id={{.String}}">{{.String}}</a>
                    </div>
                </div>
			{{end}}         
            </div>
          </li>

            <li>
                <div class="collapsible-header">Spent Inputs</div>
                <div class="collapsible-body">
			{{range .Inputs}}
                <div class="row">
                    <div class="col s12 m8">
                      <a class="truncate" href="/tx?id={{.String}}">{{.String}}</a>
                    </div>
                </div>
			{{end}}         
                </div>
              </li>
        </ul>

    </div>
  </div>

	{{template "footer" .}}
{{end}}
