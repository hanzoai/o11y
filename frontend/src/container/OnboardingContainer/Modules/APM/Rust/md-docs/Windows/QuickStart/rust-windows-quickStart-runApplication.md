
**Step 4: Set environment variables and run app**

Go to your `.env` file and update the value of required variables i.e

```bash
APP_NAME={{MYAPP}}
O11Y_ENDPOINT=https://ingest.{{REGION}}.o11y.hanzo.ai:443
O11Y_ACCESS_TOKEN={{O11Y_INGESTION_KEY}}
```

Now run `cargo run` in terminal to start the application!