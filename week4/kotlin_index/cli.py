import argparse
import json
import sys
from pathlib import Path

from .core import Embedder, build, chunk, compatible, corpus_stats, fetch, load_documents, query_input, rank, read_index
from .evaluate import compare


def parser():
    p = argparse.ArgumentParser(description='Index official Kotlin documentation with local embeddings.')
    p.add_argument('--manifest', type=Path, default=Path('corpus.json'))
    p.add_argument('--data', type=Path, default=Path('data'))
    p.add_argument('--db', type=Path, default=Path('data/index.sqlite'))
    p.add_argument('--ollama-url', default='http://127.0.0.1:11434')
    p.add_argument('--model', default='embeddinggemma:300m')
    commands = p.add_subparsers(dest='command', required=True)
    commands.add_parser('fetch', help='Download manifest sources and save normalized snapshots')
    b = commands.add_parser('build', help='Embed saved snapshots into an atomic replacement index')
    b.add_argument('--strategy', choices=['fixed', 'semantic', 'both'], default='both')
    b.add_argument('--fixed-size', type=int, default=1800)
    b.add_argument('--overlap', type=int, default=200)
    b.add_argument('--minimum', type=int, default=600)
    b.add_argument('--maximum', type=int, default=2400)
    b.add_argument('--threshold', type=float, default=0.65)
    commands.add_parser('inspect', help='Print index metadata without Ollama')
    s = commands.add_parser('search', help='Diagnostic exact cosine retrieval; does not generate answers')
    s.add_argument('query')
    s.add_argument('--strategy', choices=['fixed', 'semantic'], default='semantic')
    s.add_argument('--limit', type=int, default=5)
    s.add_argument('--platform', choices=['common', 'jvm', 'android', 'native', 'js', 'wasm'])
    s.add_argument('--json', action='store_true')
    c = commands.add_parser('compare', help='Evaluate both strategies against section labels')
    c.add_argument('--questions', type=Path, default=Path('evaluation.json'))
    c.add_argument('--output', type=Path, default=Path('reports'))
    return p


def main():
    args = parser().parse_args()
    try:
        if args.command == 'fetch':
            checksums = fetch(args.manifest, args.data / 'sources')
            Path('corpus-checksums.json').write_text(json.dumps(checksums, indent=2))
            print(json.dumps(corpus_stats(load_documents(args.manifest, args.data / 'sources')), indent=2))
            return
        if args.command == 'inspect':
            print(json.dumps(read_index(args.db)[0], indent=2))
            return
        if args.command == 'search' and args.limit <= 0:
            raise ValueError('Search limit must be positive')
        if args.command == 'build':
            config = {key: getattr(args, key) for key in ('fixed_size', 'overlap', 'minimum', 'maximum', 'threshold')}
            if not (0 <= config['overlap'] < config['fixed_size'] and 0 < config['minimum'] <= config['maximum'] and -1 <= config['threshold'] <= 1):
                raise ValueError('Invalid chunking configuration')
            documents = load_documents(args.manifest, args.data / 'sources')
        embedder = Embedder(args.ollama_url, args.model, args.data / 'embeddings.sqlite')
        try:
            if args.command == 'build':
                strategies = ['fixed', 'semantic'] if args.strategy == 'both' else [args.strategy]
                print(json.dumps(build(documents, args.db, embedder, strategies, config), indent=2))
            elif args.command == 'compare':
                print(json.dumps(compare(args.db, args.questions, embedder, args.output), indent=2))
            elif args.command == 'search':
                meta, documents, rows = read_index(args.db)
                compatible(meta, embedder)
                if not any(c['strategy'] == args.strategy for c, _ in rows):
                    raise ValueError('Strategy is absent from this index')
                allowed = None if args.platform is None else {k for k, d in documents.items() if args.platform in d['platforms'] or 'common' in d['platforms']}
                results = rank(rows, embedder.embed([query_input(args.query)])[0], args.strategy, allowed)[:args.limit]
                for result in results:
                    doc = documents[result['document_id']]
                    result.update(source=doc['url'], title=doc['title'], platforms=doc['platforms'], release_version=doc.get('release_version'), availability=doc.get('availability', 'unknown'))
                if args.json:
                    print(json.dumps(results, indent=2, ensure_ascii=False))
                else:
                    for result in results:
                        print(f"{result['score']:.3f} {result['title']} lines {result['line_start']}-{result['line_end']}\n{result['source']}\n{result['text']}\n")
        finally:
            embedder.cache.close()
    except (OSError, ValueError, KeyError) as exc:
        print('error: ' + str(exc), file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
