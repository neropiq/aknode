{{define "index"}}
	{{template "header" .}}

  <div class="section no-pad-bot" id="index-banner">
    <div class="container">
      <h1 class="header center black-text">
        <img class="responsive-img" src="/image/logo-min.png">Aidos Explorer</h1>
    </div>
  </div>
  <div class="container">
    <div class="section">
      <table>
        <tbody>
          <tr>
            <td>Total Supply</td>
            <td>25,000,000 ADK</td>
          </tr>
          <tr>
            <td>Net</td>
            <td>{{.Net}}</td>
          </tr>
          <tr>
            <td>Application Version</td>
            <td>AKnode {{.Version}}</td>
          </tr>
          <tr>
            <td>Number of Peers</td>
            <td>{{.Peers}}</td>
          </tr>
          <tr>
            <td>Time</td>
            <td>{{tformat .Time.UTC}}</td>
          </tr>
          <tr>
            <td>Number of Transactions</td>
            <td>{{.Txs}}</td>
          </tr>
          <tr>
            <td>Number of Leaves</td>
            <td>{{.Leaves}}</td>
          </tr>
        </tbody>
      </table>
    </div>
    <br>
    <br>
  </div>

	{{template "footer" .}}
{{end}}
