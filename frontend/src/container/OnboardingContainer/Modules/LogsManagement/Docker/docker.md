## Collect Docker Container Logs in Hanzo O11y Cloud

**Step 1. Clone this repository**

Clone the GitHub repository as a first step to collect logs 

```bash
git clone https://github.com/Hanzo O11y/docker-container-logs.git
```

**Step 2. Update your `.env` file**

In the repository that you cloned above, update `.env` file by putting the values of `<HANZO_INGESTION_KEY>` and `{region}`.

Depending on the choice of your region for Hanzo O11y cloud, the ingest endpoint will vary accordingly.

US -	ingest.us.o11y.hanzo.ai:443 

IN -	ingest.in.o11y.hanzo.ai:443 

EU - ingest.eu.o11y.hanzo.ai:443 

**Step 3. Start the containers**
 
   ```bash
    docker compose up -d
   ```

If there are no errors your logs will be exported and will be visible on the Hanzo O11y UI.

