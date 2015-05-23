import Ember from 'ember';
import socketMixin from 'ember-websockets/mixins/sockets';

export default Ember.Route.extend(socketMixin, {
	socketURL: "",
	beforeModel: function() {
	  console.log("OK boy");
		var parser = document.createElement('a');
		parser.href = window.location.href;
		 
		/*
		parser.pathname; // => "/pathname/"
		parser.host;     // => "example.com:3000"
		*/

		if (parser.protocol === "http:") {
			this.socketURL = "ws://"+parser.host+parser.pathname;
		} else {
			this.socketURL = "wss://"+parser.host+parser.pathname;
		}
		this.socketURL += "ws";

	  console.log("OK",this.socketURL);
	},
	model: function() { return this.store.find('index'); }
});
