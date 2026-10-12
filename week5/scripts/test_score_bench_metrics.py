import unittest
from score_bench_metrics import metrics, eligible

class MetricsTest(unittest.TestCase):
    def fixture(self):
        return ({'input_sha256':'x','passes':1,'questions':['q'], 'trials':[{
            'completed':True,'question_id':'q','duration':2_000_000_000,
            'preparation':{'scores':{'a':3,'b':0,'c':2},'candidates':[{'id':x} for x in 'abc'],'calls':[]},
            'evidence':{'chunks':[{'id':'a'}]}}]},
            {'input_sha256':'x','questions':{'q':{'a':3,'b':2,'c':0}}})
    def test_metrics(self):
        r,l=self.fixture();m=metrics(r,l)
        self.assertEqual(m['recall'],.5);self.assertEqual(m['precision'],.5)
        self.assertEqual(m['packed_recall'],.5);self.assertEqual(m['median_seconds'],2)
        self.assertLess(m['ndcg5'],1);self.assertTrue(eligible(m,m))
    def test_failure_and_binding(self):
        r,l=self.fixture();r['trials'][0]['error']='bad'
        m=metrics(r,l);self.assertEqual(m['recall'],0);self.assertEqual(m['errors'],1)
        l['input_sha256']='other'
        with self.assertRaises(ValueError):metrics(r,l)

class SelectionAuditTest(unittest.TestCase):
    def test_ranking_and_whole_chunk_budget(self):
        from score_bench_metrics import packed_ids
        def chunk(id, text):
            return dict(id=id,text=text,title='T',sections=['S'],source='u',line_start=1,line_end=1,availability='unknown')
        p={'candidates':[chunk('a','short'),chunk('b','x'*1000),chunk('c','short')], 'scores':{'a':2,'b':3,'c':3}}
        self.assertEqual(packed_ids(p,limit=2,budget=500),['c'])
        self.assertEqual(packed_ids(p,rerank=False,limit=2,budget=500),['a'])

class MetricGateTest(unittest.TestCase):
    def test_quality_boundary(self):
        baseline={'recall':.9,'packed_recall':.8}
        self.assertTrue(eligible({'errors':0,'recall':.85,'packed_recall':.75},baseline))
        self.assertFalse(eligible({'errors':0,'recall':.84,'packed_recall':.8},baseline))
        self.assertFalse(eligible({'errors':1,'recall':1,'packed_recall':1},baseline))

    def test_incomplete_cannot_qualify(self):
        run,labels=MetricsTest().fixture();run['trials'][0]['completed']=False
        with self.assertRaises(ValueError):metrics(run,labels)

class ReviewBindingTest(unittest.TestCase):
    def test_hash_alone_is_not_a_review(self):
        import importlib.util
        from pathlib import Path
        spec=importlib.util.spec_from_file_location('render_score_bench',Path(__file__).with_name('render-score-bench.py'))
        module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
        trial={'question_id':'q01','completed':True,'error':'invalid quotation'}
        review={'sha256':module.fingerprint(trial)}
        self.assertFalse(module.valid_review(review,trial))
        review.update(support='N/A',correct_unknown=False,notes='Rejected quotation.',facts=['no']*3)
        self.assertTrue(module.valid_review(review,trial))
        trial['error']='changed'
        self.assertFalse(module.valid_review(review,trial))

class FinalistSelectionTest(unittest.TestCase):
    def test_finalist_is_a_modified_configuration(self):
        import importlib.util
        from pathlib import Path
        spec=importlib.util.spec_from_file_location('run_score_bench',Path(__file__).with_name('run-score-bench.py'))
        module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
        base=dict(errors=0,recall=1,packed_recall=1,median_seconds=1)
        baseline=dict(name='baseline',scorer={},metrics=base)
        candidate=dict(name='temperature0',scorer={'temperature':0},metrics=dict(base,median_seconds=2))
        screen=dict(selected=baseline,baseline=baseline,records=[baseline,candidate])
        self.assertEqual(module.choose_finalist(screen),candidate)
