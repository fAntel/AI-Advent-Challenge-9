import hashlib
import json
from pathlib import Path
import tempfile
import unittest
from local_kotlin_index import compatible_manifest


class ManifestCompatibilityTests(unittest.TestCase):
    def test_only_metadata_extension_is_compatible(self):
        original = {'schemaVersion':2, 'config':{'digest':'config'}, 'layers':[{'digest':'weights'}]}
        expected = hashlib.sha256(json.dumps(original, separators=(',', ':')).encode()).hexdigest()
        with tempfile.TemporaryDirectory() as tmp:
            models = Path(tmp)
            (models / 'blobs').mkdir()
            def write(value):
                raw = json.dumps(value, separators=(',', ':')).encode()
                actual = hashlib.sha256(raw).hexdigest()
                (models / 'blobs' / ('sha256-' + actual)).write_bytes(raw)
                return actual
            actual = write(dict(original, runner='ggml', format='gguf'))
            self.assertTrue(compatible_manifest(actual, expected, models))
            expanded = dict(original, runner='ggml', format='gguf', layers=original['layers'] + [
                {'mediaType':'application/vnd.docker.distribution.manifest.v2+json', 'digest':'alternate-runner'}])
            raw = json.dumps(expanded, separators=(',', ':')).encode()
            tag = models / 'manifests/registry.ollama.ai/library/embeddinggemma/300m'
            tag.parent.mkdir(parents=True)
            tag.write_bytes(raw)
            self.assertTrue(compatible_manifest(hashlib.sha256(raw).hexdigest(), expected, models))
            self.assertFalse(compatible_manifest(write(dict(original, runner='llamacpp', format='gguf')), expected, models))
            changed = dict(original, runner='ggml', format='gguf', layers=[{'digest':'different-weights'}])
            self.assertFalse(compatible_manifest(write(changed), expected, models))
            (models / 'blobs' / ('sha256-' + actual)).write_text('{}')
            self.assertFalse(compatible_manifest(actual, expected, models))
            self.assertFalse(compatible_manifest('../bad', expected, models))


if __name__ == '__main__':
    unittest.main()
