How to load test a HTTP endpoint?

Part 1: Getting the targets

Part 2: Hitting the target

1. We need requests that can be hit at the target endpoint.
2. Hit the requests.

Part 3: Visualizing the hits

tail -f metrics.txt | \
 jaggr @count=rps \
          hist\[100,200,300,400,500\]:status_codes \
          p25,p50,p95:latency \
          sum:bytes_in \
          sum:bytes_out | \
    jplot rps+code.hist.100+code.hist.200+code.hist.300+code.hist.400+code.hist.500 \
          latency.p95\/1000+latency.p50\/1000+latency.p25\/1000 \
          bytes_in.sum+bytes_out.sum