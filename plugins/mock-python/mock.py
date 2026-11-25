import os
import sys
import threading
from concurrent.futures import ThreadPoolExecutor

import grpc

token = os.environ["DEMO_TOKEN"]

def monitor_parent():
    sys.stdin.buffer.read()
    os._exit(0)

if os.getenv("DEMO_PARENT_PIPE") == "1":
    threading.Thread(target=monitor_parent, daemon=True).start()

def health(request, context):
    if dict(context.invocation_metadata()).get("instance-token") != token:
        context.abort(grpc.StatusCode.UNAUTHENTICATED, "invalid instance token")
    # grpc.health.v1.HealthCheckResponse, status = SERVING or NOT_SERVING.
    return bytes((8, 2 if os.getenv("DEMO_VERSION") == "3.0.0" else 1))

with ThreadPoolExecutor(max_workers=2) as executor:
    server = grpc.server(executor)
    server.add_generic_rpc_handlers((grpc.method_handlers_generic_handler(
        "grpc.health.v1.Health", {"Check": grpc.unary_unary_rpc_method_handler(health)}
    ),))
    endpoint = os.environ["DEMO_ENDPOINT"]
    certificate = os.getenv("DEMO_IPC_CERT")
    if certificate:
        server.add_secure_port(endpoint, grpc.ssl_server_credentials((
            (os.environ["DEMO_IPC_KEY"].encode(), certificate.encode()),
        )))
    else:
        server.add_insecure_port(endpoint)
    server.start()
    server.wait_for_termination()
