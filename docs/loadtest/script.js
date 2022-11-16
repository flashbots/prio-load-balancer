import http from 'k6/http';
import { check } from "k6";
import { SharedArray } from 'k6/data';

const blockNumber = 14050699;

const data = new SharedArray('txs', function () {
    return JSON.parse(open('./transactions.json'));
});

const url = "YOUR_URL"

export default function () {
    const idx = Math.floor(Math.random() * data.length)
    const isHighPrio = Math.random() < 0.5

    console.log("req tx:", idx, "highPrio:", isHighPrio)

    var params = {
        headers: {
            'Content-Type': 'application/json',
            'high_priority': isHighPrio
        },
    };

    const payload = {
        "jsonrpc": "2.0",
        "id": 1,
        "method": "eth_callBundle",
        "params": [
            {
                "txs": [data[idx]],
                "blockNumber": `0x${(blockNumber + 10).toString(16)}`,
                "stateBlockNumber": "latest"
            }
        ]
    }

    const res = http.post(url, JSON.stringify(payload), params);
    const resData = res.json();

    check(res, { "status is 200": (r) => r.status === 200 });
    check(resData, { "response has no error": (d) => !d.error });
}
