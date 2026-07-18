import json, os, pathlib, zipfile
from datetime import datetime

test = os.environ['TEST']
test_dir = os.environ['TEST_DIR']
ext = os.environ['TEST_EXT']
artifact = os.environ['TEST_ARTIFACT']
pack_dir = os.environ['PACK_DIR']
meta_file = os.environ['META_FILE']

data = json.load(open(meta_file, encoding='utf-8'))
meta = data.get(test, {})
name = meta.get('name', test)
desc = meta.get('description', 'Brak opisu')
timestamp = datetime.utcnow().strftime('%Y-%m-%dT%H:%M:%SZ')

out = {'name': name, 'version': 1, 'description': desc, 'lastUpdated': timestamp, 'nativeBinary': True}
json.dump(out, open(os.path.join(test_dir, 'meta.json'), 'w'), indent=2, ensure_ascii=False)

pathlib.Path(os.path.join(pack_dir, f'{test}.txt')).write_text(desc, encoding='utf-8')

src = os.path.join(test_dir, f'{test}{ext}')
dst = os.path.join(pack_dir, f'{test}-{artifact}.zip')
with zipfile.ZipFile(dst, 'w', zipfile.ZIP_DEFLATED) as zf:
    zf.write(src, os.path.basename(src))
