import http from 'k6/http';
import { check } from 'k6';

export const options = {
  vus: 1,
  duration: '30s',
};

export default function () {
  const res = http.get('http://localhost/health');
  check(res, { 'health ok': (r) => r.status === 200 });
}
