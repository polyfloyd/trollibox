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
IMG_SIZE = (512, 512)


def get_mpd_library_dir(f):
	fixHome = lambda p: p.replace('~', os.getenv('HOME'))
	with open(fixHome(f), 'r') as conf:
		res = re.findall('^\\s*music_directory\\s+"(\\S+)"\\s*$', conf.read(), re.MULTILINE)
		if len(res) == 0:
			return None
		return fixHome(res[0])

def get_art_base64(f, size):
	audio_file = mutagen.File(f)
	data = None
	if not audio_file is None and 'APIC:' in audio_file.tags:
		data = audio_file.tags['APIC:'].data
	if not audio_file is None and hasattr(audio_file, 'pictures') and len(audio_file.pictures) > 0:
		data = audio_file.pictures[0].data
	if data is None:
		return None

	try:
		img = Image.open(io.BytesIO(data))
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

	total_files_updated = 0
	total_image_size    = 0

	for song in client.listallinfo(''):
		if not 'file' in song:
			continue

		file_rel = song['file']
		file_abs = os.path.join(libdir, file_rel)
		img_data = get_art_base64(file_abs, IMG_SIZE)

		if img_data is None:
			continue
		print('Updating %s, size=%s' % (file_rel, len(img_data)))

		total_files_updated += 1
		total_image_size += len(img_data)

		# If the size of the image is too big, MPD will tell us to fuck off
		# when its input buffer is saturated.
		# This has been solved by splitting the image data into smaller chunks.
		# Alternatives:
		# - Compile MPD ourselves with a bigger buffer.
		# - Modify MPD's sqlite sticker database without using MPD and hope the
		#   transmitbuffer is bigger.
		# - Start a webserver and just save an URL in the sticker
		chunks = []
		chunk_size = 6 * 1024
		for i in range(0, len(img_data), chunk_size):
			if i+chunk_size <= len(img_data):
				chunk = img_data[i:i+chunk_size]
			else:
				chunk = img_data[i:]
			chunks.append(chunk)

		client.sticker_set('song', file_rel, 'image-nchunks', str(len(chunks)))
		for (i, chunk) in enumerate(chunks):
			client.sticker_set('song', file_rel, 'image-'+str(i), chunk)

	client.close()
	client.disconnect()

	print('%s files updated' % total_files_updated)
	print('%s KB total image size' % (round(total_image_size / 1024, 2)))
