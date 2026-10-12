"""Week4 indexer with loopback-only HTTP and verified Ollama manifest compatibility."""
import hashlib
import json
import os
from pathlib import Path
import sys
import urllib.request
from urllib.parse import urlsplit


def compatible_manifest(actual, expected, models):
    """Accept only the exact old manifest plus Ollama's new runner/format fields."""
    if actual == expected:
        return True
    if len(actual) != 64 or any(c not in '0123456789abcdef' for c in actual):
        return False
    try:
        blob = models / 'blobs' / ('sha256-' + actual)
        # New Ollama versions keep the expanded manifest only at its tag path.
        raw = (blob if blob.exists() else models / 'manifests/registry.ollama.ai/library/embeddinggemma/300m').read_bytes()
        if hashlib.sha256(raw).hexdigest() != actual:
            return False
        manifest = json.loads(raw)
        if manifest.pop('runner', None) != 'ggml' or manifest.pop('format', None) != 'gguf':
            return False
        # Alternate-runner manifests are packaging metadata; retain every
        # inference/configuration layer in the digest comparison below.
        manifest['layers'] = [layer for layer in manifest['layers'] if layer.get('mediaType') not in (
            'application/vnd.ollama.manifest.list.v2+json',
            'application/vnd.docker.distribution.manifest.v2+json')]
        original = json.dumps(manifest, separators=(',', ':'), ensure_ascii=False).encode()
        return hashlib.sha256(original).hexdigest() == expected
    except (OSError, ValueError, TypeError):
        return False


def main():
    from kotlin_index import core, cli
    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, req, fp, code, msg, headers, newurl):
            raise ValueError('Local retrieval refuses HTTP redirects')
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
    def local_json(url, body=None):
        parsed = urlsplit(url)
        if parsed.scheme != 'http' or parsed.hostname != '127.0.0.1' or parsed.username or parsed.password:
            raise ValueError('Local retrieval requires loopback HTTP')
        data = None if body is None else json.dumps(body).encode()
        with opener.open(urllib.request.Request(url, data=data, headers={'Content-Type':'application/json'}), timeout=180) as response:
            return json.load(response)
    # An isolated offline server can use a different loopback port.
    if os.environ.get('OLLAMA_HOST') and '--ollama-url' not in sys.argv:
        endpoint = os.environ['OLLAMA_HOST']
        if not endpoint.startswith('http://'):
            endpoint = 'http://' + endpoint
        sys.argv[1:1] = ['--ollama-url', endpoint]
    args = cli.parser().parse_args()
    if args.command != 'search':
        raise SystemExit('The local RAG wrapper supports search only; use Week4 for indexing.')
    metadata, _, _ = core.read_index(args.db)
    expected = metadata['embedding']
    core.request_json = local_json
    show = local_json(args.ollama_url.rstrip('/') + '/api/show', {'model':args.model})
    if args.model.endswith(':cloud') or show.get('remote_host') or show.get('remote_model'):
        raise SystemExit('Local retrieval refuses remote embedding models')
    original = core.Embedder
    class LocalEmbedder(original):
        def __init__(self, *a, **kw):
            super().__init__(*a, **kw)
            actual = self.identity['digest']
            models = Path(os.environ.get('OLLAMA_MODELS', str(Path.home() / '.ollama/models')))
            if compatible_manifest(actual, expected['digest'], models):
                self.identity['digest'] = expected['digest']
            # The inherited compatibility check still rejects changed weights or settings.
    cli.Embedder = LocalEmbedder
    return cli.main()


if __name__ == '__main__':
    sys.exit(main())
