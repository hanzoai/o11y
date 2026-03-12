
You can stream logs from Heroku to Hanzo O11y using [httpsdrain](https://devcenter.heroku.com/articles/log-drains#https-drains).


&nbsp;

### Use the Heroku CLI to add a https drain


```bash
heroku drains:add https://<TENANT_NAME>:{{HANZO_INGESTION_KEY}}@ingest.{{REGION}}.o11y.hanzo.ai:443/logs/heroku -a <YOUR_APP_NAME>
```

&nbsp;

`<TENANT_NAME>` should be raplaced with the name of your Hanzo O11y instance.

For example, if your Hanzo O11y instance URL is `https://cpvo-test.us.o11y.hanzo.ai` the `TENANT_NAME` is `cpvo-test`. 


**Note:** You can find your instance URL in your browser's current tab address bar or in the invite email sent to you.

&nbsp;

`<YOUR_APP_NAME>` is the name of the Heroku application where you want to add the drain.

&nbsp;

Once you have successfully added the drain, click on the `Done` button below to see your logs in the Hanzo O11y UI.