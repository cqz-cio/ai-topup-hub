import test from 'node:test';
import assert from 'node:assert/strict';
import { randomBytes } from 'node:crypto';
import { Store } from '../server/store.mjs';
import { vault } from '../server/security.mjs';
import { Partner } from '../server/partner.mjs';
import { RedemptionService } from '../server/service.mjs';

// Match the fields observed at /api/auth/session, using invented values only.
const sessionDocument = {
  WARNING_BANNER: 'Synthetic fixture; not a real credential.',
  accessToken: 'synthetic-access-token-not-valid',
  account: { id: 'synthetic-account', planType: 'free' },
  user: { id: 'synthetic-user', email: 'session-test@example.com' },
  authProvider: 'auth0',
  expires: '2099-01-01T00:00:00.000Z',
  sessionToken: 'synthetic-session-token-not-valid',
  statsigContext: {},
  marketingEligibility: {},
};

for (const outcome of ['accepted', 'blocked', 'rejected']) {
  test(`ChatGPT Session JSON reaches partner preflight intact; ${outcome} remains unsubmitted`, async t => {
    const store = new Store(':memory:');
    t.after(() => store.close());
    const crypt = vault(randomBytes(32).toString('base64'));
    const calls = [];
    const session = JSON.stringify(sessionDocument, null, 2);
    const partner = new Partner({ password: 'synthetic-password', fetchImpl: async (url, options) => {
      const operation = new URL(url).pathname.split('/').at(-1);
      calls.push(operation);
      const body = JSON.parse(options.body);
      if (operation === 'preview') return Response.json({ ok: true, data: { redemption_token: 'synthetic-preview-token', product: 'gpt', plan: 'plus' } });
      assert.equal(operation, 'preflight', 'Account verification must never submit a recharge');
      assert.equal(body.code, 'SYNTHETIC-SOURCE-CARD');
      assert.equal(body.redemption_token, 'synthetic-preview-token');
      assert.deepEqual(body.credential, { mode: 'session', session });
      assert.deepEqual(JSON.parse(body.credential.session), sessionDocument);
      if (outcome === 'rejected') return Response.json({ ok: false, error: { message: 'Private provider rejection' } }, { status: 422 });
      return Response.json({ ok: true, data: {
        email: sessionDocument.user.email,
        currentPlan: 'free',
        preflight_token: 'synthetic-preflight-token',
        can_redeem: outcome === 'accepted',
      } });
    } });
    const service = new RedemptionService({ store, vault: crypt, partner, publicOrigin: 'https://redeem.example.test' });
    const card = service.issue([{ reference: 'session-shape-test', source_code: 'SYNTHETIC-SOURCE-CARD' }])[0];
    const device = 'synthetic-browser-device-123456789';
    const preview = await service.preview(card.code, device);
    if (outcome === 'rejected') {
      await assert.rejects(service.preflight(preview.attempt_id, session, device), error => error.code === 'UPSTREAM_REJECTED');
    } else {
      const checked = await service.preflight(preview.attempt_id, session, device);
      assert.equal(checked.state, outcome === 'accepted' ? 'checked' : 'blocked');
      assert.equal(checked.email, sessionDocument.user.email);
      assert.equal(checked.submitted, false);
      assert.equal(Boolean(checked.confirmation), outcome === 'accepted');
      assert.ok(!JSON.stringify(checked).includes(sessionDocument.accessToken));
      assert.ok(!JSON.stringify(checked).includes(sessionDocument.sessionToken));
    }
    assert.deepEqual(calls, ['preview', 'preflight']);
    const attempt = store.get('SELECT * FROM attempts WHERE id=?', preview.attempt_id);
    assert.equal(attempt.session_enc, '', 'GPT Session must not persist after verification');
    assert.ok(!attempt.submitted_at);
    assert.equal(store.get('SELECT status FROM cards WHERE id=?', card.id).status, 'ready');
    assert.ok(!JSON.stringify(service.data(attempt)).includes(sessionDocument.sessionToken));
    assert.equal(attempt.state, outcome === 'accepted' ? 'checked' : outcome === 'blocked' ? 'blocked' : 'preview');
  });
}
