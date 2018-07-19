{{define "tx"}}
	{{template "header" .}}

  <div class="container">
    <div class="section">
      <div class="center">
        <h4>Transaction</h4>
        <a class="truncate" href="/tx?hash={{.TXID}}">{{.TXID}}</a>
        <div>Created on {{.Created.UTC.String}}</div>
        <div>Received on {{.Received.UTC.String}}</div>
{{if eq .Status 0xff}}
        <div class="rejected">Rejected</div>
{{end}}
{{if eq .Status 0x01}}
        <div class="confirmed">Confirmed</div>
{{end}}
{{if eq .Status 0x00}}
        <div class="pending">Pending</div>
{{end}}
        <div class="row">
          <div class="col s12 m5">
{{if ne (len .Inputs) 0}}
            <h5>Inputs</h5>
{{range .Inputs}}
            <div class="tx small-address truncate">
              <a href="/address?id={{.Address.String}}">{{.Address.String}}</a>
              <span class="right amount">{{toADK .Value}} ADK</span>
            </div>
{{end}}
{{end}}
{{if ne (len .MInputs) 0}}
            <h5>Multisig Inputs</h5>
{{range .MInputs}}
            <div class="tx small-address truncate">
              {{.N}} out of {{len .Address}}
              <br>
			{{range .Address}}
			{{if .HasSign}}
              <i class="red-text text-lighten-2 material-icons prefix">check</i>
			{{end}}
              <a href="/address?id={{.Address.String}}">{{.Address.String}}</a>
              <br>
			{{end}}
             <span class="right amount">{{.Value}} ADK</span>
            </div>
{{end}}
{{end}}
          </div>

          <div class="col s1">
            <h5></h5>
            <div class="">
              <i class="material-icons center-align valign-wrapper">arrow_forward</i>
            </div>
          </div>

          <div class="col s12 m5">
{{if ne (len .Outputs) 0}}
            <h5>Outputs</h5>
{{range .Outputs}}
            <div class="tx small-address truncate">
              <a href="/address?id={{.Address.String}}">{{.Address.String}}</a>
              <span class="right amount">{{toADK .Value}} ADK</span>
            </div>
{{end}}
{{end}}
{{if ne (len .MOutputs) 0}}
            <h5>Multisig Outputs</h5>
{{range .MOutputs}}
            <div class="tx small-address truncate">
              {{.N}} out of {{len .Address}}
              <br>
{{range .Address}}
              <a href="/address?id={{.String}}">{{.Address.String}}</a>
{{end}}
             <span class="right amount">{{toADK .Value}} ADK</span>
            </div>
{{end}}
{{end}}
          </div>
        </div>
      </div>
    </div>
  </div>

{{if and .TicketInput .TicketOutput }}
  <div class="container">
    <div class="row">
      <div class="col s12 m5">
        <h5>Ticket Inputs</h5>
        <div class="tx small-address truncate">
          <a href="/address?id={{.TicketInput.String}}">{{.TicketInput.String}}</a>
        </div>
      </div>
      <div class="col s1">
        <h5></h5>
        <i class="material-icons center-align valign-wrapper">arrow_forward</i>
      </div>
      <div class="col s12 m5">
        <h5>Ticket Outputs</h5>
        <div class="tx small-address truncate">
          <span>
          <a href="/address?id={{.TicketOutput.String}}">{{.TicketOutput.String}}</a>
          </span>
        </div>
      </div>
    </div>
  </div>
{{end}}

  <div class="container">
    <div class="section">
      <ul class="collapsible">
        <li>
          <div class="collapsible-header">
            Message
          </div>
          <div class="collapsible-body">
            <p class="truncate">{{.Message}}</p>
            <p class="truncate">("{{.MessageStr}}")</p>
          </div>
        </li>
        <li>
            <div class="collapsible-header">
            Nonce
          </div>
          <div class="collapsible-body">
{{range .Nonce}}
            <div>{{.}}<br/></div>
{{end}}
          </div>
        <li>
          <div class="collapsible-header">
            GNonce
          </div>
          <div class="collapsible-body">
            <p class="truncate">{{.GNonce}}</p>
          </div>
          </li>
          <li>
          <div class="collapsible-header">
            Locktime
          </div>
          <div class="collapsible-body">
            <p class="truncate">{{tformat .LockTime.UTC}}</p>
          </div>
        </li>
        <li>
          <div class="collapsible-header">
            Parents
          </div>
          <div class="collapsible-body">
{{range .Parents}}
            <a class="truncate" href="/tx?id={{.String}}">{{.String}}</a>
{{end}}
          </div>
        </li>
      </ul>
    </div>
  </div>

{{template "footer" .}}
{{end}}
