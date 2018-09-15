{{define "address"}}
	{{template "header" .}}

  <div class="container">
    <div class="section">
      <div class="center">
        <h4>Address</h4>
        <img src="/qrcode?id={{.Address}}">
        <a class="truncate" href="/address?id={{.Address}}">{{.Address}}</a>
        <a class="balance">Balance: {{toADK .Balance}} ADK</a>
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
              <div class="collapsible-header">UTXOs(Multisig)</div>
              <div class="collapsible-body">
			{{range .MUTXOs}}
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
            <li>
                <div class="collapsible-header">Spent Inputs(Multisig)</div>
                <div class="collapsible-body">
			{{range .MInputs}}
                <div class="row">
                    <div class="col s12 m8">
                      <a class="truncate" href="/tx?id={{.String}}">{{.String}}</a>
                    </div>
                </div>
			{{end}}         
                </div>
              </li>
          <li>
                <div class="collapsible-header">Unspent Ticketout</div>
                <div class="collapsible-body">
			{{range .Ticketouts}}
                <div class="row">
                    <div class="col s12 m8">
                      <a class="truncate" href="/tx?id={{.String}}">{{.String}}</a>
                    </div>
                </div>
			{{end}}         
                </div>
              </li>

          <li>
                <div class="collapsible-header">Spent Ticket Inputs</div>
                <div class="collapsible-body">
			{{range .Ticketins}}
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
