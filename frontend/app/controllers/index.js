import Ember from 'ember';

var stateMapping = ["Pending", "Error", "Timeout", "Success"];

export default Ember.ArrayController.extend({
	logcount: 0,
	showstats: false,
	actions: {
		onopen: function(socketEvent) {
			console.log('Websocket opened', socketEvent);
		},
		onmessage: function(evt) {
			//console.log('Websocket msg', evt);
			var m = JSON.parse(evt.data);
			var now = moment().format('HH:mm:ss');
			if ( m.messageType && m.messageType === "log") {
				Ember.$("#log").prepend("<li><span class='logdate'>"+ now +"</span> "+m.messageData+"</li>");
				this.logcount++;
			} else if ( m.messageType && m.messageType === "error") {
				Ember.$("#log").prepend("<li><span class='logdate'>"+ now +"</span> <span class='error'>"+m.messageData+"</span></li>");
				this.logcount++;
			} else if ( m.messageType && m.messageType === "fatal") {
				Ember.$("#log").prepend("<li><span class='logdate'>"+ now +"</span> <span class='error'>"+m.messageData+"</span></li>");
				this.logcount++;
			} else if ( m.messageType && m.messageType === "device") {
				var pending = false;
				if (m.status.State===0) {
					pending = true;
				}
				var error = false;
				if (m.status.State===1 || m.status.StateIsError===1) {
					error = true;
				}
				this.store.push('index', {
					"id": m.device,
					"hostname": m.status.Device.Hostname,
					"isPending": pending,
					"isError": error,
					"state": stateMapping[m.status.State]
				});
			} else if ( (m.messageType && m.messageType === "metric") && (this.get("showstats")===true) ) {
				if (m.device === "goroutines") {
					Ember.$("#numgoroutines").html(m.messageData);
				} else {
					console.log(m);
				}
		  }
			if (this.logcount > 80) {
				Ember.$("#log li").last().remove();
				this.logcount--;
			}
		},
		onclose: function() {
			Ember.$("#devicestatus").html("<h2 class='error'>Network error - please reload.</h2>");
			var now = moment().format('HH:mm:ss');
			Ember.$("#log").prepend("<li><span class='logdate'>"+ now +"</span> <span class='error'>Network error - server connection closed.</span></li>");
		},
		statstoggle: function() {
			if (this.get("showstats")===true) {
				this.set("showstats", false);
			} else {
				this.set("showstats", true);
			}
		}
	}
});


