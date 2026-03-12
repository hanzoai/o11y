
**Step 4: Set environment variables and run app**

Go to your `.env` file and update the value of required variables i.e

```bash
APP_NAME={{MYAPP}}
HANZO_ENDPOINT=https://ingest.{{REGION}}.o11y.hanzo.ai:443
HANZO_ACCESS_TOKEN={{HANZO_INGESTION_KEY}}
```

Now run `cargo run` in terminal to start the application!