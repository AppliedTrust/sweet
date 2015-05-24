import Ember from 'ember';
import socketMixin from 'ember-websockets/mixins/sockets';

export default Ember.Route.extend(socketMixin, {
	socketURL: "",
	beforeModel: function() {
		var parser = document.createElement('a');
		parser.href = window.location.href;
		if (parser.protocol === "http:") {
			this.socketURL = "ws://"+parser.host+parser.pathname;
		} else {
			this.socketURL = "wss://"+parser.host+parser.pathname;
		}
		this.socketURL += "ws";
	},
	model: function() { return this.store.find('index'); }
});
