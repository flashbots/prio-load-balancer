https://k6.io/docs/getting-started/running-k6/

Update `blockNumber` in `script.js`

```bash
# Run the script 1x
k6 run script.js

# Run 10 parallel request loops, for 30 sec total
k6 run --vus 10 --duration 30s script.js
```
