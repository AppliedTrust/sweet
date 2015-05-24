import DS from 'ember-data';

export default DS.Model.extend({
  hostname: DS.attr('string'),
  state: DS.attr('string'),
  isPending: DS.attr('bool'),
  isError: DS.attr('bool'),
  errorMessage: DS.attr('string')
});
