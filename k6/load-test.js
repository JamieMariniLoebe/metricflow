import http from 'k6/http'
import { sleep, check } from 'k6'

http.setResponseCallback(http.expectedStatuses(202, 503))

export const options = {
    stages: [
        { duration: '30s', target: 10 },
        { duration: '2m', target: 10 },
        { duration: '30s', target: 50 },
        { duration: '2m', target: 50 },
        { duration: '30s', target: 200 },
        { duration: '2m', target: 200 },
        { duration: '30s', target: 0 }
    ],
    thresholds: {
        http_req_duration: ['p(95)<25'],
        checks: ['rate>0.99']
    },
};

export default function () { 
    const url = 'http://localhost:8080/api/metrics'
    const payload = JSON.stringify({"metric_name":"cpu_usage", "metric_type":"gauge","measured_at": new Date().toISOString()})
    const params = { headers: { 'Content-Type': 'application/json' },
                     tags: { endpoint: 'ingest' }
    }

    const res = http.post(url, payload, params)

    check(res, {
        'expected status (202 or 503)': (r) => r.status === 202 || r.status === 503
    })

    sleep(.5)
}