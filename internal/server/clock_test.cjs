// Run the actual inline clock script with a controlled clock and minimal DOM.
const { test } = require('node:test');
const assert = require('node:assert/strict');
const { readFileSync } = require('node:fs');
const { join } = require('node:path');
const vm = require('node:vm');

const template = readFileSync(join(__dirname, 'templates/base.html'), 'utf8');
const script = template.match(/<script>([\s\S]*?)<\/script>/)[1];

for (const scenario of [
  {
    name: 'positive offset crosses into a new year', offset: 19800,
    before: '2024-12-31T18:29:59Z', after: '2024-12-31T18:30:00Z',
    beforeDate: 'Tuesday, 31 December 2024', afterDate: 'Wednesday, 1 January 2025',
  },
  {
    name: 'negative sub-hour offset crosses into a new month', offset: -1800,
    before: '2024-03-01T00:29:59Z', after: '2024-03-01T00:30:00Z',
    beforeDate: 'Thursday, 29 February 2024', afterDate: 'Friday, 1 March 2024',
  },
]) {
  test(scenario.name, () => {
    let instant = Date.parse(scenario.before);
    let tick;
    const date = { textContent: 'server-rendered date' };
    const clock = {
      textContent: '',
      hasAttribute: () => false,
      getAttribute: name => name === 'data-offset' ? String(scenario.offset) : null,
      parentElement: { querySelector: () => date },
    };
    class ControlledDate extends Date {
      constructor(...args) { super(...(args.length ? args : [instant])); }
    }
    vm.runInNewContext(script, {
      Date: ControlledDate, Intl,
      document: { querySelectorAll: () => [clock] },
      setInterval: callback => { tick = callback; },
    });
    assert.equal(clock.textContent, '23:59:59');
    assert.equal(date.textContent, scenario.beforeDate);
    instant = Date.parse(scenario.after);
    tick();
    assert.equal(clock.textContent, '00:00:00');
    assert.equal(date.textContent, scenario.afterDate);
  });
}
