#! /usr/bin/env python3

# Requires: pillow, python-mpd2, mutagen

from PIL import Image
from mpd import MPDClient
import base64
import io
import mutagen
import os
import re

MPD_HOST = 'localhost'
MPD_PORT = 6600
MPD_CONF = '~/.mpdconf'
IMG_SIZE = (120, 120)


def get_mpd_library_dir(f):
	fixHome = lambda p: p.replace('~', os.getenv('HOME'))
	with open(fixHome(f), 'r') as conf:
		res = re.findall('^\\s*music_directory\\s+"(\\S+)"\\s*$', conf.read(), re.MULTILINE)
		if len(res) == 0:
			return None
		return fixHome(res[0])

def get_art_base64(f, size):
	audio_file = mutagen.File(f)
	if audio_file is None or not 'APIC:' in audio_file.tags:
		return None
	try:
		img = Image.open(io.BytesIO(audio_file.tags['APIC:'].data))
		img.thumbnail(size)
		buf = io.BytesIO()
		img.save(buf, "JPEG")
		return base64.b64encode(buf.getvalue()).decode('utf-8')
	except:
		return None # Whatever, just ignore it


if __name__ == '__main__':
	client = MPDClient()
	client.timeout = 10
	client.idletimeout = None
	client.connect(MPD_HOST, MPD_PORT)

	libdir = get_mpd_library_dir(MPD_CONF)

	for song in client.listallinfo(''):
		if not 'file' in song:
			continue

		file_rel = song['file']
		file_abs = os.path.join(libdir, file_rel)
		img_data = get_art_base64(file_abs, IMG_SIZE)

		if img_data is None:
			print('Skipping %s' % file_rel)
			continue
		print('Updating %s, size=%s' % (file_rel, len(img_data)))

		# TODO: Increase the image size without shitting up the MPD network connection.
		# If the size of the image is too big, MPD will tell us to fuck off when its input buffer is saturated.
		# We could get around this issue:
		# - Split the image across multiple stickers.
		# - Compile MPD ourselves with a bigger buffer.
		# - Modify MPD's sqlite sticker database without using MPD and hope the
		#   transmitbuffer is bigger.
		# - Start a webserver and just save an URL in the sticker
		client.sticker_set('song', file_rel, 'image', img_data)
		client.sticker_set('song', file_rel, 'has-image', '1')

	client.close()
	client.disconnect()
